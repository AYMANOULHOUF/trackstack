// Package notify delivers tracking alerts to the channels an organization
// has enabled in notification_settings: email and Web Push. Dashboard
// notifications already travel over the WebSocket firehose, so they are not
// re-sent here.
package notify

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Config holds delivery credentials. Email and Web Push are independent: if
// one is not configured its channel is skipped and the reason is logged.
type Config struct {
	SMTPAddr     string // host:port, e.g. smtp.example.com:587
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
	VAPIDPublic  string
	VAPIDPrivate string
	VAPIDSubject string // mailto:you@example.com
}

// Event is one alert to deliver to every enabled channel of an organization.
type Event struct {
	OrgID      string
	Kind       string // speed | geofence_enter | geofence_exit
	Title      string
	Body       string
	DeviceID   string
	DeviceName string
	OccurredAt time.Time
}

// Dispatcher serializes deliveries through a small queue so position ingest
// never waits on SMTP or a push endpoint.
type Dispatcher struct {
	db     *sql.DB
	cfg    Config
	queue  chan Event
	logger *log.Logger
}

func NewDispatcher(db *sql.DB, cfg Config) *Dispatcher {
	if cfg.VAPIDSubject == "" {
		cfg.VAPIDSubject = "mailto:admin@trackstack.local"
	}
	return &Dispatcher{
		db:     db,
		cfg:    cfg,
		queue:  make(chan Event, 256),
		logger: log.Default(),
	}
}

// VAPIDPublicKey is exposed to the dashboard so browsers can subscribe.
func (d *Dispatcher) VAPIDPublicKey() string { return d.cfg.VAPIDPublic }

// Enqueue never blocks; a full queue drops the alert with a log line.
func (d *Dispatcher) Enqueue(ev Event) {
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now().UTC()
	}
	select {
	case d.queue <- ev:
	default:
		d.logger.Printf("notify: queue full, dropped %s for org %s", ev.Kind, ev.OrgID)
	}
}

// Run consumes the queue until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-d.queue:
			d.deliver(ctx, ev)
		}
	}
}

type setting struct {
	channel string
	target  string
}

func (d *Dispatcher) deliver(parent context.Context, ev Event) {
	settings, err := d.settings(parent, ev.OrgID)
	if err != nil {
		d.logger.Printf("notify: load settings for org %s: %v", ev.OrgID, err)
		return
	}
	if len(settings) == 0 {
		return
	}

	for _, s := range settings {
		ctx, cancel := context.WithTimeout(parent, 15*time.Second)
		switch s.channel {
		case "email":
			if d.cfg.SMTPAddr == "" || d.cfg.SMTPFrom == "" {
				d.logger.Printf("notify: email enabled but SMTP is not configured")
				cancel()
				continue
			}
			if s.target == "" {
				d.logger.Printf("notify: email enabled without a target address")
				cancel()
				continue
			}
			if err := d.sendEmail(ctx, s.target, ev); err != nil {
				d.logger.Printf("notify: email to %s failed: %v", s.target, err)
			}
		case "web_push":
			if d.cfg.VAPIDPrivate == "" || d.cfg.VAPIDPublic == "" {
				d.logger.Printf("notify: web_push enabled but VAPID keys are not configured")
				cancel()
				continue
			}
			if s.target == "" {
				cancel()
				continue
			}
			if err := d.sendWebPush(ctx, s.target, ev); err != nil {
				d.logger.Printf("notify: web push failed: %v", err)
			}
		}
		cancel()
	}
}

func (d *Dispatcher) settings(ctx context.Context, orgID string) ([]setting, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT channel, COALESCE(target, '')
		FROM notification_settings
		WHERE org_id = $1 AND enabled AND user_id IS NULL`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []setting{}
	for rows.Next() {
		var s setting
		if err := rows.Scan(&s.channel, &s.target); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *Dispatcher) sendEmail(ctx context.Context, to string, ev Event) error {
	from := d.cfg.SMTPFrom
	host, _, err := net.SplitHostPort(d.cfg.SMTPAddr)
	if err != nil {
		host = d.cfg.SMTPAddr
	}

	var conn net.Conn
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if strings.HasSuffix(d.cfg.SMTPAddr, ":465") {
		conn, err = tls.DialWithDialer(dialer, "tcp", d.cfg.SMTPAddr, &tls.Config{ServerName: host})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", d.cfg.SMTPAddr)
	}
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("client: %w", err)
	}
	defer client.Close()

	if !strings.HasSuffix(d.cfg.SMTPAddr, ":465") {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	if d.cfg.SMTPUser != "" {
		auth := smtp.PlainAuth("", d.cfg.SMTPUser, d.cfg.SMTPPass, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}

	body := ev.Title + "\r\n\r\n" + ev.Body + "\r\n\r\n" +
		"Device: " + ev.DeviceName + " (" + ev.DeviceID + ")\r\n" +
		"Time: " + ev.OccurredAt.UTC().Format(time.RFC3339) + "\r\n"
	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + ev.Title + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n" +
		"\r\n" + body
	if _, err := w.Write([]byte(msg)); err != nil {
		w.Close()
		return fmt.Errorf("write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return client.Quit()
}

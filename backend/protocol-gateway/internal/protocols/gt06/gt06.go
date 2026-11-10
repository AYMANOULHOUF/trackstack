// Package gt06 implements the TCP side of the GT06/Concox tracker family.
//
// The adapter understands the two frames needed for tracking:
//   - 0x01 login, which carries the 8-byte BCD IMEI
//   - 0x12 location, which carries UTC time, latitude/longitude, speed and course
//
// Other frame types (heartbeat, status) are acknowledged and ignored. The IMEI
// is forwarded as external_id; an administrator links it to a device in the
// dashboard.
package gt06

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/trackproj/protocol-gateway/internal/forwarder"
)

const (
	protocolLogin    byte = 0x01
	protocolLocation byte = 0x12
)

var (
	ErrShortFrame = errors.New("short frame")
	ErrStart      = errors.New("invalid start marker")
	ErrStop       = errors.New("invalid stop marker")
	ErrChecksum   = errors.New("checksum mismatch")
)

// Frame is a decoded GT06 frame.
type Frame struct {
	Protocol byte
	Serial   uint16
	Info     []byte
}

// Location is a decoded 0x12 payload.
type Location struct {
	Lat        float64
	Lon        float64
	Speed      *float64
	Heading    *float64
	RecordedAt time.Time
	HasFix     bool
}

// ParseFrame decodes one frame from data and returns the bytes consumed.
func ParseFrame(data []byte) (Frame, int, error) {
	if len(data) < 5 {
		return Frame{}, 0, ErrShortFrame
	}
	if data[0] != 0x78 || data[1] != 0x78 {
		return Frame{}, 0, ErrStart
	}
	length := int(data[2])
	if length < 3 {
		return Frame{}, 0, ErrShortFrame
	}
	total := 2 + 1 + length + 2
	if len(data) < total {
		return Frame{}, 0, ErrShortFrame
	}
	if data[total-2] != 0x0D || data[total-1] != 0x0A {
		return Frame{}, 0, ErrStop
	}

	content := data[3 : 3+length]
	check := data[2]
	for _, b := range content[:len(content)-1] {
		check ^= b
	}
	if check != content[len(content)-1] {
		return Frame{}, 0, ErrChecksum
	}

	f := Frame{
		Protocol: content[0],
		Serial:   binary.BigEndian.Uint16(content[len(content)-3 : len(content)-1]),
		Info:     content[1 : len(content)-3],
	}
	return f, total, nil
}

// ReadFrame reads one frame from a TCP stream.
func ReadFrame(r *bufio.Reader) (Frame, error) {
	head := make([]byte, 3)
	if _, err := io.ReadFull(r, head); err != nil {
		return Frame{}, err
	}
	if head[0] != 0x78 || head[1] != 0x78 {
		return Frame{}, ErrStart
	}
	rest := make([]byte, int(head[2])+2)
	if _, err := io.ReadFull(r, rest); err != nil {
		return Frame{}, err
	}
	raw := append(append([]byte{}, head...), rest...)
	f, _, err := ParseFrame(raw)
	return f, err
}

// ParseIMEI decodes the 8-byte BCD IMEI from a login frame.
func ParseIMEI(info []byte) (string, error) {
	if len(info) < 8 {
		return "", fmt.Errorf("login info is %d bytes, want 8", len(info))
	}
	var b strings.Builder
	for _, x := range info[:8] {
		b.WriteByte('0' + (x >> 4))
		b.WriteByte('0' + (x & 0x0F))
	}
	imei := b.String()
	if len(imei) == 16 && imei[0] == '0' {
		imei = imei[1:]
	}
	return imei, nil
}

// ParseLocation decodes a 0x12 payload.
func ParseLocation(info []byte) (Location, error) {
	if len(info) < 1 {
		return Location{}, ErrShortFrame
	}
	dtLen := int(info[0])
	if dtLen < 6 || len(info) < 1+dtLen+1 {
		return Location{}, ErrShortFrame
	}
	dt := info[1 : 1+dtLen]
	recordedAt := time.Date(
		2000+int(dt[0]), time.Month(dt[1]), int(dt[2]),
		int(dt[3]), int(dt[4]), int(dt[5]), 0, time.UTC,
	)

	pos := 1 + dtLen
	gpsFlag := info[pos]
	pos++
	if gpsFlag == 0 {
		return Location{RecordedAt: recordedAt}, nil
	}
	if len(info) < pos+11 {
		return Location{}, ErrShortFrame
	}

	latRaw := int32(binary.BigEndian.Uint32(info[pos : pos+4]))
	lonRaw := int32(binary.BigEndian.Uint32(info[pos+4 : pos+8]))
	speed := float64(info[pos+8])
	course := binary.BigEndian.Uint16(info[pos+9 : pos+11])

	lat := float64(latRaw) / 1800000.0
	lon := float64(lonRaw) / 1800000.0
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return Location{}, fmt.Errorf("coordinates out of range: %.6f,%.6f", lat, lon)
	}
	heading := float64(course & 0x03FF)

	return Location{
		Lat: lat, Lon: lon, Speed: &speed, Heading: &heading,
		RecordedAt: recordedAt, HasFix: true,
	}, nil
}

// BuildAck creates the server response for a client frame. status 0 means
// accepted, 1 means rejected.
func BuildAck(protocol byte, serial uint16, status byte) []byte {
	content := []byte{protocol, status, byte(serial >> 8), byte(serial)}
	length := byte(len(content) + 1)
	check := length
	for _, b := range content {
		check ^= b
	}
	out := []byte{0x78, 0x78, length}
	out = append(out, content...)
	out = append(out, check, 0x0D, 0x0A)
	return out
}

// Server accepts tracker connections and forwards fixes to api-service.
type Server struct {
	Addr      string
	Forwarder *forwarder.Forwarder
	Logger    *log.Logger
}

func NewServer(addr string, fwd *forwarder.Forwarder) *Server {
	return &Server{Addr: addr, Forwarder: fwd, Logger: log.Default()}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("gt06 listen: %w", err)
	}
	s.Logger.Printf("gt06 adapter listening on %s", s.Addr)

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("gt06 accept: %w", err)
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	imei := ""

	for {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		frame, err := ReadFrame(reader)
		if err != nil {
			return
		}

		switch frame.Protocol {
		case protocolLogin:
			id, err := ParseIMEI(frame.Info)
			status := byte(0)
			if err != nil {
				status = 1
			} else {
				imei = id
			}
			_, _ = conn.Write(BuildAck(frame.Protocol, frame.Serial, status))

		case protocolLocation:
			if imei == "" {
				_, _ = conn.Write(BuildAck(frame.Protocol, frame.Serial, 1))
				continue
			}
			loc, err := ParseLocation(frame.Info)
			if err != nil || !loc.HasFix {
				_, _ = conn.Write(BuildAck(frame.Protocol, frame.Serial, 1))
				continue
			}

			fctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = s.Forwarder.Forward(fctx, forwarder.Position{
				ExternalID: imei,
				Lat:        loc.Lat,
				Lon:        loc.Lon,
				Speed:      loc.Speed,
				Heading:    loc.Heading,
				RecordedAt: loc.RecordedAt,
			})
			cancel()

			status := byte(0)
			if err != nil {
				status = 1
				s.Logger.Printf("gt06: forward %s failed: %v", imei, err)
			}
			_, _ = conn.Write(BuildAck(frame.Protocol, frame.Serial, status))

		default:
			_, _ = conn.Write(BuildAck(frame.Protocol, frame.Serial, 0))
		}
	}
}

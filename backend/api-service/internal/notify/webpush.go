package notify

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"
)

// pushSubscription is the JSON a browser returns from
// PushManager.subscribe(). It is stored in notification_settings.target.
type pushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (d *Dispatcher) sendWebPush(ctx context.Context, target string, ev Event) error {
	var sub pushSubscription
	if err := json.Unmarshal([]byte(target), &sub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}
	if sub.Endpoint == "" || sub.Keys.P256DH == "" || sub.Keys.Auth == "" {
		return fmt.Errorf("subscription is missing endpoint or keys")
	}

	message := ev.Title + "\n\n" + ev.Body
	payload, err := encryptWebPush(message, sub)
	if err != nil {
		return err
	}
	auth, err := d.vapidAuth(sub.Endpoint)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build push request: %w", err)
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "86400")
	req.Header.Set("Authorization", auth)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("push request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("push endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// encryptWebPush implements RFC 8291 (aes128gcm) for a single record.
func encryptWebPush(message string, sub pushSubscription) ([]byte, error) {
	uaPubBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.P256DH, "="))
	if err != nil {
		return nil, fmt.Errorf("decode p256dh: %w", err)
	}
	authSecret, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.Auth, "="))
	if err != nil {
		return nil, fmt.Errorf("decode auth: %w", err)
	}

	curve := ecdh.P256()
	uaPub, err := curve.NewPublicKey(uaPubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse p256dh: %w", err)
	}
	eph, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	shared, err := eph.ECDH(uaPub)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	// RFC 8291 §3.4: combine the ECDH secret with the subscription auth secret,
	// then bind both public keys into the info string.
	prk := hkdf.Extract(sha256.New, shared, authSecret)
	info := append([]byte("WebPush: info\x00"), uaPubBytes...)
	info = append(info, eph.PublicKey().Bytes()...)
	ikm := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, info), ikm); err != nil {
		return nil, fmt.Errorf("hkdf ikm: %w", err)
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}
	prk2 := hkdf.Extract(sha256.New, ikm, salt)

	cek := make([]byte, 16)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk2, []byte("Content-Encoding: aes128gcm\x00")), cek); err != nil {
		return nil, fmt.Errorf("hkdf cek: %w", err)
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk2, []byte("Content-Encoding: nonce\x00")), nonce); err != nil {
		return nil, fmt.Errorf("hkdf nonce: %w", err)
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	plain := []byte(message)
	if len(plain) > 3000 {
		plain = plain[:3000]
	}
	// 0x02 marks the final (only) record in the aes128gcm content coding.
	record := append(append([]byte{}, plain...), 0x02)
	ciphertext := gcm.Seal(nil, nonce, record, nil)

	header := make([]byte, 0, 16+4+1+65)
	header = append(header, salt...)
	rs := make([]byte, 4)
	binary.BigEndian.PutUint32(rs, 4096)
	header = append(header, rs...)
	header = append(header, byte(65))
	header = append(header, eph.PublicKey().Bytes()...)

	return append(header, ciphertext...), nil
}

// vapidAuth builds the RFC 8292 Authorization header.
func (d *Dispatcher) vapidAuth(endpoint string) (string, error) {
	priv, pub, err := parseVAPIDKeys(d.cfg.VAPIDPrivate, d.cfg.VAPIDPublic)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	aud := u.Scheme + "://" + u.Host

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims := fmt.Sprintf(`{"aud":%q,"exp":%d,"sub":%q}`,
		aud, time.Now().Add(12*time.Hour).Unix(), d.cfg.VAPIDSubject)
	signingInput := header + "." + base64.RawURLEncoding.EncodeToString([]byte(claims))

	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign vapid jwt: %w", err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])

	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
	return "vapid t=" + token + ", k=" + base64.RawURLEncoding.EncodeToString(pub), nil
}

func parseVAPIDKeys(privateB64, publicB64 string) (*ecdsa.PrivateKey, []byte, error) {
	dBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(privateB64, "="))
	if err != nil || len(dBytes) != 32 {
		return nil, nil, fmt.Errorf("VAPID private key must be 32 base64url bytes")
	}

	ecdhPriv, err := ecdh.P256().NewPrivateKey(dBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse VAPID private key: %w", err)
	}
	pub := ecdhPriv.PublicKey().Bytes()

	if publicB64 != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(publicB64, "="))
		if err != nil || len(decoded) != 65 {
			return nil, nil, fmt.Errorf("VAPID public key must be 65 base64url bytes")
		}
		pub = decoded
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), pub)
	if x == nil || y == nil {
		return nil, nil, fmt.Errorf("invalid VAPID public key point")
	}
	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y},
		D:         new(big.Int).SetBytes(dBytes),
	}
	return priv, pub, nil
}

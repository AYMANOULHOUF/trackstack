package notify

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"strings"
	"testing"

	"golang.org/x/crypto/hkdf"
)

func newBigInt(b []byte) *big.Int { return new(big.Int).SetBytes(b) }

func TestVAPIDAuthIsVerifiable(t *testing.T) {
	dBytes := make([]byte, 32)
	if _, err := rand.Read(dBytes); err != nil {
		t.Fatal(err)
	}
	d := &Dispatcher{cfg: Config{
		VAPIDPrivate: base64.RawURLEncoding.EncodeToString(dBytes),
		VAPIDSubject: "mailto:test@example.com",
	}}

	auth, err := d.vapidAuth("https://push.example.com/send/abc")
	if err != nil {
		t.Fatalf("vapidAuth: %v", err)
	}
	if !strings.HasPrefix(auth, "vapid t=") {
		t.Fatalf("unexpected header: %s", auth)
	}
	parts := strings.Split(strings.TrimPrefix(auth, "vapid t="), ", k=")
	if len(parts) != 2 {
		t.Fatalf("unexpected header: %s", auth)
	}
	token := parts[0]

	jwtParts := strings.Split(token, ".")
	if len(jwtParts) != 3 {
		t.Fatalf("jwt has %d parts", len(jwtParts))
	}

	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(jwtParts[1])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Aud != "https://push.example.com" {
		t.Fatalf("aud = %q", claims.Aud)
	}
	if claims.Sub != "mailto:test@example.com" {
		t.Fatalf("sub = %q", claims.Sub)
	}

	priv, _, err := parseVAPIDKeys(d.cfg.VAPIDPrivate, "")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(jwtParts[2])
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature length = %d", len(sig))
	}
	digest := sha256.Sum256([]byte(jwtParts[0] + "." + jwtParts[1]))
	if !ecdsa.Verify(&priv.PublicKey, digest[:], newBigInt(sig[:32]), newBigInt(sig[32:])) {
		t.Fatal("VAPID JWT signature did not verify")
	}
}

func TestEncryptWebPushRoundTrip(t *testing.T) {
	uaPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatal(err)
	}

	sub := pushSubscription{Endpoint: "https://push.example.com/send/abc"}
	sub.Keys.P256DH = base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes())
	sub.Keys.Auth = base64.RawURLEncoding.EncodeToString(authSecret)

	payload, err := encryptWebPush("Speed limit exceeded", sub)
	if err != nil {
		t.Fatalf("encryptWebPush: %v", err)
	}
	if len(payload) < 16+4+1+65+16 {
		t.Fatalf("payload too short: %d", len(payload))
	}

	salt := payload[:16]
	rs := binary.BigEndian.Uint32(payload[16:20])
	if rs != 4096 {
		t.Fatalf("record size = %d", rs)
	}
	idLen := int(payload[20])
	ephPubBytes := payload[21 : 21+idLen]
	ciphertext := payload[21+idLen:]

	ephPub, err := ecdh.P256().NewPublicKey(ephPubBytes)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := uaPriv.ECDH(ephPub)
	if err != nil {
		t.Fatal(err)
	}

	prk := hkdf.Extract(sha256.New, shared, authSecret)
	info := append([]byte("WebPush: info\x00"), uaPriv.PublicKey().Bytes()...)
	info = append(info, ephPubBytes...)
	ikm := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, info), ikm); err != nil {
		t.Fatal(err)
	}
	prk2 := hkdf.Extract(sha256.New, ikm, salt)
	cek := make([]byte, 16)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk2, []byte("Content-Encoding: aes128gcm\x00")), cek); err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk2, []byte("Content-Encoding: nonce\x00")), nonce); err != nil {
		t.Fatal(err)
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if len(plain) == 0 || plain[len(plain)-1] != 0x02 {
		t.Fatalf("missing final record marker: %v", plain)
	}
	if got := string(plain[:len(plain)-1]); got != "Speed limit exceeded" {
		t.Fatalf("plaintext = %q", got)
	}
}

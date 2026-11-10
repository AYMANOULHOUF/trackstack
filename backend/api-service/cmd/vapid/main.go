// Command vapid prints a fresh VAPID key pair for Web Push.
//
// Usage:
//
//	go run ./cmd/vapid
//
// Copy the two values into VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY. The public
// key is also served to the dashboard through GET /v1/notifications so the
// browser can subscribe.
package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
)

func main() {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("generate key: %v", err)
	}

	// RFC 8292 uses the uncompressed point (65 bytes) as the public key and the
	// 32-byte scalar as the private key, both base64url without padding.
	pub := priv.PublicKey().Bytes()
	fmt.Printf("VAPID_PUBLIC_KEY=%s\n", base64.RawURLEncoding.EncodeToString(pub))
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", base64.RawURLEncoding.EncodeToString(priv.Bytes()))
	fmt.Println("VAPID_SUBJECT=mailto:you@example.com")
}

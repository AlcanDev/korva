// keygen generates a fresh RSA-4096 key pair for the Korva licensing server.
//
// Usage:
//
//	go run github.com/alcandev/korva/forge/licensing-server/keygen
//
// Output:
//
//	priv.pem   — private key (NEVER commit; set as KORVA_LICENSING_PRIVATE_KEY_PEM on server)
//	pubkey.pem — public key  (copy to internal/license/keys/pubkey.pem, then rebuild binaries)
//
// Run this command once before going live to replace the placeholder key.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	fmt.Println("Generating RSA-4096 key pair…")

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating key: %v\n", err)
		os.Exit(1)
	}

	// Marshal private key as PKCS#8 (modern standard).
	privDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal private key: %v\n", err)
		os.Exit(1)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	if err := os.WriteFile("priv.pem", privPEM, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "write priv.pem: %v\n", err)
		os.Exit(1)
	}

	// Marshal public key as PKIX/SPKI (standard format for go:embed).
	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal public key: %v\n", err)
		os.Exit(1)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	if err := os.WriteFile("pubkey.pem", pubPEM, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write pubkey.pem: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Copy pubkey.pem → internal/license/keys/pubkey.pem")
	fmt.Println("  2. Rebuild all binaries (the new public key will be embedded)")
	fmt.Println("  3. On the licensing server, set:")
	fmt.Println("       export KORVA_LICENSING_PRIVATE_KEY_FILE=/path/to/priv.pem")
	fmt.Println("  4. Delete priv.pem from this machine — it should only exist on the server")
	fmt.Println()
	fmt.Println("SECURITY: priv.pem must NEVER be committed to the repository.")
}

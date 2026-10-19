// Command mfsign creates and checks the Ed25519 signatures used by V2
// releases: the release's checksums.txt, and (with the same kind of key)
// task manifests.
//
//	mfsign genkey                         prints a new key pair (base64)
//	mfsign sign <file>                    signs with $MFSIGN_KEY, prints the signature
//	mfsign verify <pubkey> <file> <sig>   exits 0 only if the signature is good
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "genkey":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		check(err)
		fmt.Printf("public:  %s\nprivate: %s\n", base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv))
	case "sign":
		if len(os.Args) != 3 {
			usage()
		}
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("MFSIGN_KEY")))
		if err != nil || len(key) != ed25519.PrivateKeySize {
			fail("MFSIGN_KEY must hold a base64 Ed25519 private key")
		}
		msg, err := os.ReadFile(os.Args[2])
		check(err)
		fmt.Println(base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), msg)))
	case "verify":
		if len(os.Args) != 5 {
			usage()
		}
		pub, err := base64.StdEncoding.DecodeString(os.Args[2])
		if err != nil || len(pub) != ed25519.PublicKeySize {
			fail("bad public key")
		}
		msg, err := os.ReadFile(os.Args[3])
		check(err)
		sigText, err := os.ReadFile(os.Args[4])
		check(err)
		sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
		if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
			fail("signature does NOT match")
		}
		fmt.Println("signature OK")
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: mfsign genkey | sign <file> | verify <pubkey> <file> <sigfile>")
	os.Exit(2)
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "mfsign:", msg)
	os.Exit(1)
}

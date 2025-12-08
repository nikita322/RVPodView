package updater

import (
	"fmt"

	"github.com/jedisct1/go-minisign"
)

// PublicKeyStr is the minisign public key for verifying releases (base64 only)
const PublicKeyStr = "RWTs/v2+ntUcvpgj3hhtLesiIv6ny153HNmYsGvzrkVbCCy8lHHKo5Mv"

// VerifySignature verifies the archive signature using minisign
func VerifySignature(archivePath, signaturePath string, pubKey minisign.PublicKey) error {
	// Read signature file
	sig, err := minisign.NewSignatureFromFile(signaturePath)
	if err != nil {
		return fmt.Errorf("read signature file: %w", err)
	}

	// Verify the archive
	valid, err := pubKey.VerifyFromFile(archivePath, sig)
	if err != nil {
		return fmt.Errorf("verify signature: %w", err)
	}

	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// ParsePublicKey parses the embedded public key
func ParsePublicKey(keyStr string) (minisign.PublicKey, error) {
	return minisign.NewPublicKey(keyStr)
}

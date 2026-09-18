package integrationauth

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	HashAlgorithmBcrypt = "bcrypt"
	BcryptCost          = 12
)

func HashSecret(plaintext string) (hash string, err error) {
	out, err := bcrypt.GenerateFromPassword([]byte(plaintext), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash secret: %w", err)
	}
	return string(out), nil
}

func VerifySecret(plaintext, secretHash, algorithm string) error {
	if strings.TrimSpace(algorithm) != HashAlgorithmBcrypt {
		return fmt.Errorf("unsupported hash algorithm")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(secretHash), []byte(plaintext)); err != nil {
		return fmt.Errorf("invalid secret")
	}
	return nil
}

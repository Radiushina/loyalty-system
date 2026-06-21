package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

type Encryptor struct {
	secret []byte
}

func New(secret string) *Encryptor {
	return &Encryptor{secret: []byte(secret)}
}

var ErrInvalidValue = errors.New("invalid value")

func (e *Encryptor) Encrypt(value string) (string, error) {
	block, err := aes.NewCipher(e.secret)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	return string(aesGCM.Seal(nonce, nonce, []byte(value), nil)), nil
}

func (e *Encryptor) Decrypt(encryptedValue string) (string, error) {
	block, err := aes.NewCipher(e.secret)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	nonceSize := aesGCM.NonceSize()
	if len(encryptedValue) < nonceSize {
		return "", fmt.Errorf("decrypt: %w", ErrInvalidValue)
	}

	nonce := encryptedValue[:nonceSize]
	ciphertext := encryptedValue[nonceSize:]

	value, err := aesGCM.Open(nil, []byte(nonce), []byte(ciphertext), nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", ErrInvalidValue)
	}
	return string(value), nil
}

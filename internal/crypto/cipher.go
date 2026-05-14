package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const secretsPurpose = "server_secrets"

type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(master *MasterKey) (*Cipher, error) {
	sub, err := master.Derive(secretsPurpose)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(sub[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: aes init: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: gcm init: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (b *Cipher) Seal(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	return b.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (b *Cipher) Open(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}
	ns := b.aead.NonceSize()
	if len(ciphertext) < ns {
		return "", errors.New("crypto: ciphertext too short")
	}
	nonce, body := ciphertext[:ns], ciphertext[ns:]
	pt, err := b.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: open: %w", err)
	}
	return string(pt), nil
}

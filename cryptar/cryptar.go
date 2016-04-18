// Package cryptar implements the crypto doing dinosaur.
package cryptar

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
)

// Encrypt takes the contents of a tarball and a key and seals it.
func Encrypt(tarball, key []byte) (string, error) {
	gcm, err := keyToGCM(key)
	if err != nil {
		return "", err
	}

	nonce, err := generateNonce(gcm.NonceSize())
	if err != nil {
		return "", err
	}

	out := gcm.Seal(nonce, nonce, tarball, nil)

	enctar := base64.StdEncoding.EncodeToString(out)

	return enctar, nil
}

// Decrypt takes an encrypted tarball and key and unseals it.
func Decrypt(enctar string, key []byte) ([]byte, error) {
	out, err := base64.StdEncoding.DecodeString(enctar)
	if err != nil {
		return nil, err
	}

	gcm, err := keyToGCM(key)
	if err != nil {
		return nil, err
	}

	size := gcm.NonceSize()
	nonce := make([]byte, size)
	copy(nonce, out[:])

	tarball, err := gcm.Open(nil, nonce, out[size:], nil)
	if err != nil {
		return nil, err
	}

	return tarball, nil
}

func keyToGCM(key []byte) (cipher.AEAD, error) {
	// encrypt the tar with the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm, err
}

// generateNonce creates a new random nonce.
func generateNonce(size int) ([]byte, error) {
	// Create the nonce.
	nonce, err := randBytes(size)
	if err != nil {
		return nil, err
	}

	return nonce, nil
}

// randBytes returns n bytes of random data.
func randBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

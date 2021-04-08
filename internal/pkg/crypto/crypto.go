package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateVMK generates 64 bytes Volume Master Key.
func GenerateVMK() ([]byte, error) {
	return GenerateRandomBytes(64)
}

// GenerateIV generates 32 bytes Initialization Vector.
func GenerateIV() ([]byte, error) {
	return GenerateRandomBytes(32)
}

// GenerateUserKey generates 32 bytes User Key.
func GenerateUserKey() ([]byte, error) {
	return GenerateRandomBytes(32)
}

// CreateIK generates 32 bytes Intermediate Key, given userKey and
// initialization vector. This would be used as key in AES-256 CBC encryption.
func CreateIK(userKey, iv []byte) ([]byte, error) {
	hash := sha256.New
	hkdfReader := hkdf.New(hash, userKey, iv, nil)

	key := make([]byte, 32)
	_, err := io.ReadFull(hkdfReader, key)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// Encrypt returns a cipher text by encrypting plaintext with the given key
// using AES-256 CBC encryption.
func Encrypt(plaintext, key []byte) ([]byte, error) {
	if len(plaintext)%aes.BlockSize != 0 {
		return nil, errors.New("plaintext is not a multiple of the block size")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// Decrypt returns a plaintext by decrypting a AES-256 CBC encrypted ciphertext
// using the given key .
func Decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	plaintext := make([]byte, len(ciphertext))

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	return plaintext, nil
}

// CreateHMAC takes a message and a key and returns HMAC of the message.
func CreateHMAC(message, key []byte) ([]byte, error) {
	h := hmac.New(sha256.New, key)
	if _, err := h.Write(message); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// CheckHMAC reports whether messageMAC is a valid HMAC tag for message.
func CheckHMAC(message, messageMAC, key []byte) (bool, error) {
	expectedMAC, err := CreateHMAC(message, key)
	if err != nil {
		return false, err
	}
	return hmac.Equal(messageMAC, expectedMAC), nil
}

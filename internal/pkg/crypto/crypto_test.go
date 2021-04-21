package crypto

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

func expect(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Errorf("Expected %v (type %v) - Got %v (type %v)", b, reflect.TypeOf(b), a, reflect.TypeOf(a))
	}
}

func TestCrypto(t *testing.T) {
	vmk, err := GenerateVMK()
	expect(t, err, nil)
	expect(t, len(vmk), 64) // Should be 64 bytes long.

	userKey, err := GenerateUserKey()
	expect(t, err, nil)
	expect(t, len(userKey), 32) // Should be 32 bytes long.

	iv, err := GenerateIV()
	expect(t, err, nil)
	expect(t, len(iv), 32) // Should be 32 bytes long.

	ik, err := CreateIK(userKey, iv)
	expect(t, err, nil)
	expect(t, len(ik), 32) // Should be 32 bytes long.

	// Generate VUK.
	vuk, err := Encrypt(vmk, ik)
	expect(t, err, nil)

	vmac, err := CreateHMAC(vmk, iv)
	expect(t, err, nil)
	expect(t, len(vmac), 32) // HMAC of 64 bytes and 32 bytes should be 32 bytes.

	// Derive VMK.
	dvmk, err := Decrypt(vuk, ik)
	expect(t, err, nil)
	expect(t, len(dvmk), 64) // Derived master key should be 64 bytes long.

	if !bytes.Equal(vmk, dvmk) {
		t.Errorf("expected the derived key to be equal to the master key:\n\t(GOT): %v\n\t(WNT): %v", dvmk, vmk)
	}

	// Verify HMAC of the derived VMK.
	hmacResult, err := CheckHMAC(dvmk, vmac, iv)
	expect(t, err, nil)

	if !hmacResult {
		t.Errorf("expected the HMAC of derived key to be equal to the HMAC of master key")
	}
}

func TestEncrypt(t *testing.T) {
	testcases := []struct {
		name      string
		key       []byte
		plaintext []byte
		wantErr   error
	}{
		{
			name:      "valid plaintext length",
			key:       make([]byte, 16),
			plaintext: make([]byte, 16),
		},
		{
			name:      "invalid plaintext length",
			key:       make([]byte, 16),
			plaintext: make([]byte, 10),
			wantErr:   errors.New("plaintext is not a multiple of the block size"),
		},
		{
			name:      "invalid key length",
			key:       make([]byte, 20),
			plaintext: make([]byte, 32),
			wantErr:   errors.New("crypto/aes: invalid key size 20"),
		},
	}

	for _, tc := range testcases {
		var tc = tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := Encrypt(tc.plaintext, tc.key)
			if err != nil {
				if tc.wantErr != nil {
					if err.Error() != tc.wantErr.Error() {
						t.Errorf("unexpected error:\n\t(GOT): %v\n\t(WNT): %v", err, tc.wantErr)
					}
				} else {
					t.Errorf("expected no error, but got error: %v", err)
				}
			} else {
				if tc.wantErr != nil {
					t.Error("expected error, but got none")
				}
			}
		})
	}
}

func TestDecrypt(t *testing.T) {
	testcases := []struct {
		name       string
		key        []byte
		ciphertext []byte
		wantErr    error
	}{
		{
			name:       "valid ciphertext length",
			key:        make([]byte, 16),
			ciphertext: make([]byte, 16),
		},
		{
			name:       "short ciphertext length",
			key:        make([]byte, 16),
			ciphertext: make([]byte, 10),
			wantErr:    errors.New("ciphertext too short"),
		},
		{
			name:       "invalid ciphertext length",
			key:        make([]byte, 16),
			ciphertext: make([]byte, 20),
			wantErr:    errors.New("ciphertext is not a multiple of the block size"),
		},
		{
			name:       "invalid key length",
			key:        make([]byte, 20),
			ciphertext: make([]byte, 32),
			wantErr:    errors.New("crypto/aes: invalid key size 20"),
		},
	}

	for _, tc := range testcases {
		var tc = tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decrypt(tc.ciphertext, tc.key)
			if err != nil {
				if tc.wantErr != nil {
					if err.Error() != tc.wantErr.Error() {
						t.Errorf("unexpected error:\n\t(GOT): %v\n\t(WNT): %v", err, tc.wantErr)
					}
				} else {
					t.Errorf("expected no error, but got error: %v", err)
				}
			} else {
				if tc.wantErr != nil {
					t.Error("expected error, but got none")
				}
			}
		})
	}
}

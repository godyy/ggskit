package aes

import (
	"bytes"
	stdaes "crypto/aes"
	"errors"
	"testing"

	"github.com/godyy/ggskit/base/crypto"
)

func TestGenerateKey_ValidSizes(t *testing.T) {
	sizes := []int{16, 24, 32}
	for _, sz := range sizes {
		key, err := GenerateKey(sz)
		if err != nil {
			t.Fatalf("GenerateKey(%d) error: %v", sz, err)
		}
		if len(key) != sz {
			t.Fatalf("GenerateKey(%d) length=%d", sz, len(key))
		}
	}
}

func TestGenerateKey_InvalidSize(t *testing.T) {
	if _, err := GenerateKey(20); err == nil {
		t.Fatalf("expected ErrKeySize for invalid size")
	} else if !errors.Is(err, ErrKeySize) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateNonceAndIV(t *testing.T) {
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce error: %v", err)
	}
	if len(nonce) != 12 {
		t.Fatalf("nonce length=%d, want=12", len(nonce))
	}

	iv, err := GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV error: %v", err)
	}
	if len(iv) != stdaes.BlockSize {
		t.Fatalf("iv length=%d, want=%d", len(iv), stdaes.BlockSize)
	}
}

func TestAESGCMCrypto_EncryptDecrypt(t *testing.T) {
	key, err := GenerateKey(32)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce error: %v", err)
	}

	gcm, err := NewAESGCMCryptor(key, nonce)
	if err != nil {
		t.Fatalf("NewAESGCMCrypto error: %v", err)
	}
	if gcm.Mode() != "GCM" {
		t.Fatalf("Mode=%s, want=GCM", gcm.Mode())
	}
	if !bytes.Equal(gcm.Key(), key) {
		t.Fatalf("Key mismatch")
	}

	original := []byte("Hello, World! This is GCM test.")
	ct, err := gcm.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	pt, err := gcm.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if !bytes.Equal(pt, original) {
		t.Fatalf("decrypted mismatch")
	}
}

func TestAESGCMCrypto_InvalidNonceSize(t *testing.T) {
	key, _ := GenerateKey(16)
	badNonce := []byte("too-short") // 9 bytes, should be 12
	if _, err := NewAESGCMCryptor(key, badNonce); err == nil {
		t.Fatalf("expected ErrGCMNonceSize for bad nonce length")
	} else if !errors.Is(err, ErrGCMNonceSize) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAESGCMCrypto_DecryptShortCiphertext(t *testing.T) {
	key, _ := GenerateKey(16)
	nonce, _ := GenerateNonce()
	gcm, err := NewAESGCMCryptor(key, nonce)
	if err != nil {
		t.Fatalf("NewAESGCMCrypto error: %v", err)
	}
	short := []byte("short")
	if _, err := gcm.Decrypt(short); err == nil {
		t.Fatalf("expected ErrEncryptedDataLength for short ciphertext")
	} else if !errors.Is(err, crypto.ErrEncryptedDataLength) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAESCBCCrypto_EncryptDecrypt(t *testing.T) {
	key, err := GenerateKey(32)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	iv, err := GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV error: %v", err)
	}

	cbc, err := NewAESCBCCryptor(key, iv)
	if err != nil {
		t.Fatalf("NewAESCBCCrypto error: %v", err)
	}
	if cbc.Mode() != "CBC" {
		t.Fatalf("Mode=%s, want=CBC", cbc.Mode())
	}
	if !bytes.Equal(cbc.Key(), key) {
		t.Fatalf("Key mismatch")
	}

	original := []byte("Hello, World! This is CBC test.")
	ct, err := cbc.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	pt, err := cbc.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if !bytes.Equal(pt, original) {
		t.Fatalf("decrypted mismatch")
	}
}

func TestAESCBCCrypto_InvalidKeyIV(t *testing.T) {
	badKey := []byte("short-key") // len=9
	iv := make([]byte, stdaes.BlockSize)
	if _, err := NewAESCBCCryptor(badKey, iv); err == nil {
		t.Fatalf("expected ErrKeySize for invalid key length")
	} else if !errors.Is(err, ErrKeySize) {
		t.Fatalf("unexpected error: %v", err)
	}

	key, _ := GenerateKey(16)
	badIV := []byte("short-iv")
	if _, err := NewAESCBCCryptor(key, badIV); err == nil {
		t.Fatalf("expected ErrCBCIVSize for invalid iv length")
	} else if !errors.Is(err, ErrCBCIVSize) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAESCBCCrypto_DecryptInvalidLength(t *testing.T) {
	key, _ := GenerateKey(16)
	iv := make([]byte, stdaes.BlockSize)
	cbc, err := NewAESCBCCryptor(key, iv)
	if err != nil {
		t.Fatalf("NewAESCBCCrypto error: %v", err)
	}
	bad := []byte("invalid") // not multiple of 16
	if _, err := cbc.Decrypt(bad); err == nil {
		t.Fatalf("expected ErrEncryptedDataLength for invalid CBC ciphertext length")
	} else if !errors.Is(err, crypto.ErrEncryptedDataLength) {
		t.Fatalf("unexpected error: %v", err)
	}
}

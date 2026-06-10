package rsa

import (
	"bytes"
	stdcrypto "crypto"
	"errors"
	"os"
	"testing"

	"github.com/godyy/ggskit/base/crypto"
)

func TestGenPrivateKey(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}
	if priv == nil || priv.N.BitLen() != 1024 {
		t.Fatalf("unexpected key generated, bitlen=%d", priv.N.BitLen())
	}
}

func TestPEMPublicKey_PKIX(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}
	pub := &priv.PublicKey

	pemBytes, err := MarshalPublicKeyPem(pub, crypto.PEMPublicKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyPem(PUBLIC KEY) error: %v", err)
	}

	parsed, err := ParsePublicKeyFromPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePublicKeyFromPEM error: %v", err)
	}

	if pub.E != parsed.E || pub.N.Cmp(parsed.N) != 0 {
		t.Fatalf("parsed public key mismatch")
	}
}

func TestPEMPublicKey_PKCS1(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}
	pub := &priv.PublicKey

	pemBytes, err := MarshalPublicKeyPem(pub, crypto.PEMRSAPublicKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyPem(RSA PUBLIC KEY) error: %v", err)
	}

	parsed, err := ParsePublicKeyFromPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePublicKeyFromPEM error: %v", err)
	}

	if pub.E != parsed.E || pub.N.Cmp(parsed.N) != 0 {
		t.Fatalf("parsed public key mismatch")
	}
}

func TestPEMPrivateKey_PKCS8(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}

	pemBytes, err := MarshalPrivateKeyPEM(priv, crypto.PEMPrivateKey)
	if err != nil {
		t.Fatalf("MarshalPrivateKeyPEM(PRIVATE KEY) error: %v", err)
	}

	parsed, err := ParsePrivateKeyFromPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyFromPEM error: %v", err)
	}

	if priv.E != parsed.E || priv.N.Cmp(parsed.N) != 0 || priv.D.Cmp(parsed.D) != 0 {
		t.Fatalf("parsed private key mismatch")
	}
}

func TestPEMPrivateKey_PKCS1(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}

	pemBytes, err := MarshalPrivateKeyPEM(priv, crypto.PEMRSAPrivateKey)
	if err != nil {
		t.Fatalf("MarshalPrivateKeyPEM(RSA PRIVATE KEY) error: %v", err)
	}

	parsed, err := ParsePrivateKeyFromPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyFromPEM error: %v", err)
	}

	if priv.E != parsed.E || priv.N.Cmp(parsed.N) != 0 || priv.D.Cmp(parsed.D) != 0 {
		t.Fatalf("parsed private key mismatch")
	}
}

func TestParseKeyFromPEMFile(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}
	pub := &priv.PublicKey

	pubPEM, err := MarshalPublicKeyPem(pub, crypto.PEMPublicKey)
	if err != nil {
		t.Fatalf("MarshalPublicKeyPem error: %v", err)
	}
	priPEM, err := MarshalPrivateKeyPEM(priv, crypto.PEMPrivateKey)
	if err != nil {
		t.Fatalf("MarshalPrivateKeyPEM error: %v", err)
	}

	pubFile, err := os.CreateTemp("", "pub-*.pem")
	if err != nil {
		t.Fatalf("CreateTemp pub error: %v", err)
	}
	defer os.Remove(pubFile.Name())
	if _, err := pubFile.Write(pubPEM); err != nil {
		t.Fatalf("write pub pem error: %v", err)
	}
	pubFile.Close()

	priFile, err := os.CreateTemp("", "pri-*.pem")
	if err != nil {
		t.Fatalf("CreateTemp pri error: %v", err)
	}
	defer os.Remove(priFile.Name())
	if _, err := priFile.Write(priPEM); err != nil {
		t.Fatalf("write pri pem error: %v", err)
	}
	priFile.Close()

	parsedPub, err := ParsePublicKeyFromPEMFile(pubFile.Name())
	if err != nil {
		t.Fatalf("ParsePublicKeyFromPEMFile error: %v", err)
	}
	parsedPri, err := ParsePrivateKeyFromPEMFile(priFile.Name())
	if err != nil {
		t.Fatalf("ParsePrivateKeyFromPEMFile error: %v", err)
	}

	if pub.E != parsedPub.E || pub.N.Cmp(parsedPub.N) != 0 {
		t.Fatalf("parsed public key from file mismatch")
	}
	if priv.E != parsedPri.E || priv.N.Cmp(parsedPri.N) != 0 || priv.D.Cmp(parsedPri.D) != 0 {
		t.Fatalf("parsed private key from file mismatch")
	}
}

func TestEncryptDecrypt_SmallAndLarge(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}
	pub := &priv.PublicKey

	enc := NewRSAEncryptor(pub)
	dec := NewRSADecryptor(priv)

	// small db
	small := []byte("hello world!")
	cipherSmall, err := enc.Encrypt(small)
	if err != nil {
		t.Fatalf("Encrypt small error: %v", err)
	}
	plainSmall, err := dec.Decrypt(cipherSmall)
	if err != nil {
		t.Fatalf("Decrypt small error: %v", err)
	}
	if !bytes.Equal(plainSmall, small) {
		t.Fatalf("small db mismatch")
	}

	// large db (multiple chunks)
	large := make([]byte, 5000)
	for i := range large {
		large[i] = byte(i % 251)
	}
	cipherLarge, err := enc.Encrypt(large)
	if err != nil {
		t.Fatalf("Encrypt large error: %v", err)
	}
	plainLarge, err := dec.Decrypt(cipherLarge)
	if err != nil {
		t.Fatalf("Decrypt large error: %v", err)
	}
	if !bytes.Equal(plainLarge, large) {
		t.Fatalf("large db mismatch")
	}
}

func TestDecrypt_InvalidLength(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}
	dec := NewRSADecryptor(priv)

	invalid := make([]byte, priv.Size()*2+1)
	if _, err := dec.Decrypt(invalid); err == nil {
		t.Fatalf("expected error for invalid length, got nil")
	} else if !errors.Is(err, crypto.ErrEncryptedDataLength) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSignVerify(t *testing.T) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		t.Fatalf("GenPrivateKey error: %v", err)
	}
	pub := &priv.PublicKey

	signer := NewRSASigner(priv, stdcrypto.SHA256)
	verifier := NewRSAVerifier(pub, stdcrypto.SHA256)

	data := []byte("sign this")
	sig, err := signer.Sign(data)
	if err != nil {
		t.Fatalf("Sign error: %v", err)
	}
	if err := verifier.Verify(data, sig); err != nil {
		t.Fatalf("Verify error: %v", err)
	}

	tampered := []byte("sign this!")
	if err := verifier.Verify(tampered, sig); err == nil {
		t.Fatalf("expected verify failure for tampered db")
	}
}

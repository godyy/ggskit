package aes

import (
	"testing"
)

var benchSink []byte

func BenchmarkAESGCMEncrypt_64B(b *testing.B) {
	key, _ := GenerateKey(32)
	nonce, _ := GenerateNonce()
	gcm, err := NewAESGCMCryptor(key, nonce)
	if err != nil {
		b.Fatalf("NewAESGCMCrypto error: %v", err)
	}
	data := make([]byte, 64)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, err := gcm.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = ct
	}
}

func BenchmarkAESGCMEncrypt_1KB(b *testing.B) {
	key, _ := GenerateKey(32)
	nonce, _ := GenerateNonce()
	gcm, err := NewAESGCMCryptor(key, nonce)
	if err != nil {
		b.Fatalf("NewAESGCMCrypto error: %v", err)
	}
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, err := gcm.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = ct
	}
}

func BenchmarkAESGCMEncrypt_64KB(b *testing.B) {
	key, _ := GenerateKey(32)
	nonce, _ := GenerateNonce()
	gcm, err := NewAESGCMCryptor(key, nonce)
	if err != nil {
		b.Fatalf("NewAESGCMCrypto error: %v", err)
	}
	data := make([]byte, 64*1024)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, err := gcm.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = ct
	}
}

func BenchmarkAESGCMDecrypt_1KB(b *testing.B) {
	key, _ := GenerateKey(32)
	nonce, _ := GenerateNonce()
	gcm, err := NewAESGCMCryptor(key, nonce)
	if err != nil {
		b.Fatalf("NewAESGCMCrypto error: %v", err)
	}
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	ct, err := gcm.Encrypt(data)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(ct)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt, err := gcm.Decrypt(ct)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = pt
	}
}

func BenchmarkAESCBCEncrypt_1KB(b *testing.B) {
	key, _ := GenerateKey(32)
	iv, _ := GenerateIV()
	cbc, err := NewAESCBCCryptor(key, iv)
	if err != nil {
		b.Fatalf("NewAESCBCCrypto error: %v", err)
	}
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, err := cbc.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = ct
	}
}

func BenchmarkAESCBCDecrypt_1KB(b *testing.B) {
	key, _ := GenerateKey(32)
	iv, _ := GenerateIV()
	cbc, err := NewAESCBCCryptor(key, iv)
	if err != nil {
		b.Fatalf("NewAESCBCCrypto error: %v", err)
	}
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	ct, err := cbc.Encrypt(data)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(ct)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt, err := cbc.Decrypt(ct)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = pt
	}
}

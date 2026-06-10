package rsa

import (
	stdcrypto "crypto"
	"testing"
)

var benchSink []byte

func BenchmarkRSAEncrypt_1KB_1024(b *testing.B) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		b.Fatal(err)
	}
	enc := NewRSAEncryptor(&priv.PublicKey)
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, err := enc.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = ct
	}
}

func BenchmarkRSADecrypt_1KB_1024(b *testing.B) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		b.Fatal(err)
	}
	enc := NewRSAEncryptor(&priv.PublicKey)
	dec := NewRSADecryptor(priv)
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	ct, err := enc.Encrypt(data)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(ct)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt, err := dec.Decrypt(ct)
		if err != nil {
			b.Fatal(err)
		}
		benchSink = pt
	}
}

func BenchmarkRSASignVerify_1024(b *testing.B) {
	priv, err := GenPrivateKey(1024)
	if err != nil {
		b.Fatal(err)
	}
	pub := &priv.PublicKey
	signer := NewRSASigner(priv, stdcrypto.SHA256)
	verifier := NewRSAVerifier(pub, stdcrypto.SHA256)
	msg := make([]byte, 256)
	for i := range msg {
		msg[i] = byte(i)
	}

	b.Run("Sign", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sig, err := signer.Sign(msg)
			if err != nil {
				b.Fatal(err)
			}
			benchSink = sig
		}
	})

	sig, err := signer.Sign(msg)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("Verify", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := verifier.Verify(msg, sig); err != nil {
				b.Fatal(err)
			}
		}
	})
}

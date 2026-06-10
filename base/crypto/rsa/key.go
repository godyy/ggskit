package rsa

import (
	"crypto/rand"
	"crypto/rsa"
)

// GenPrivateKey 生成RSA私钥.
func GenPrivateKey(keySize int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, keySize)
}

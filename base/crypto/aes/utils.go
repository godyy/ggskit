package aes

import (
	"crypto/aes"
	"crypto/rand"
	"errors"

	pkgerrors "github.com/pkg/errors"
)

// checkKeySize 检查密钥长度是否有效
func checkKeySize(keySize int) error {
	if keySize != 16 && keySize != 24 && keySize != 32 {
		return ErrKeySize
	}
	return nil
}

// GenerateKey 生成AES密钥
func GenerateKey(keySize int) ([]byte, error) {
	if err := checkKeySize(keySize); err != nil {
		return nil, err
	}

	key := make([]byte, keySize)
	_, err := rand.Read(key)
	if err != nil {
		return nil, pkgerrors.WithMessagef(err, "generate random key failed, keySize=%d", keySize)
	}
	return key, nil
}

func randBytes(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	return b, err
}

// GenerateNonce 生成GCM nonce
func GenerateNonce() ([]byte, error) {
	return randBytes(12)
}

// GenerateIV 生成初始化向量
func GenerateIV() ([]byte, error) {
	return randBytes(aes.BlockSize)
}

// pkcs7Pad PKCS7填充
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

// pkcs7Unpad PKCS7去填充
func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	padding := int(data[length-1])
	if padding > length || padding == 0 {
		return nil, errors.New("invalid padding")
	}

	for i := length - padding; i < length; i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:length-padding], nil
}

package rsa

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/godyy/ggskit/base/crypto"
	pkgerrors "github.com/pkg/errors"
)

// MarshalPublicKeyPem 将RSA公钥转换为PEM格式
func MarshalPublicKeyPem(pubKey *rsa.PublicKey, blockType string) ([]byte, error) {
	var pubKeyBytes []byte
	switch blockType {
	case crypto.PEMPublicKey:
		var err error
		pubKeyBytes, err = x509.MarshalPKIXPublicKey(pubKey)
		if err != nil {
			return nil, err
		}
	case crypto.PEMRSAPublicKey:
		pubKeyBytes = x509.MarshalPKCS1PublicKey(pubKey)
	default:
		return nil, fmt.Errorf("invalid pem block type: %s", blockType)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: pubKeyBytes})
	return pubKeyPEM, nil
}

// ParsePublicKeyFromPEM 从PEM格式解析公钥
func ParsePublicKeyFromPEM(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	switch block.Type {
	case crypto.PEMPublicKey:
		pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaPubKey, ok := pubKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return rsaPubKey, nil
	case crypto.PEMRSAPublicKey:
		return x509.ParsePKCS1PublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("invalid pem block type: %s", block.Type)
	}
}

// ParsePublicKeyFromPEMFile 从PEM文件解析公钥
func ParsePublicKeyFromPEMFile(filePath string) (*rsa.PublicKey, error) {
	pemData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "failed to read PEM file")
	}

	return ParsePublicKeyFromPEM(pemData)
}

// MarshalPrivateKeyPEM 将RSA私钥转换为PEM格式
func MarshalPrivateKeyPEM(priKey *rsa.PrivateKey, blockType string) ([]byte, error) {
	var priKeyBytes []byte
	switch blockType {
	case crypto.PEMPrivateKey:
		var err error
		priKeyBytes, err = x509.MarshalPKCS8PrivateKey(priKey)
		if err != nil {
			return nil, err
		}
	case crypto.PEMRSAPrivateKey:
		priKeyBytes = x509.MarshalPKCS1PrivateKey(priKey)
	default:
		return nil, fmt.Errorf("invalid pem block type: %s", blockType)
	}

	priKeyPEM := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: priKeyBytes})
	return priKeyPEM, nil
}

// ParsePrivateKeyFromPEM 从PEM格式解析私钥
func ParsePrivateKeyFromPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}

	switch block.Type {
	case crypto.PEMPrivateKey:
		privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaPrivKey, ok := privKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
		return rsaPrivKey, nil
	case crypto.PEMRSAPrivateKey:
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("invalid pem block type: %s", block.Type)
	}
}

// ParsePrivateKeyFromPEMFile 从PEM文件解析私钥
func ParsePrivateKeyFromPEMFile(filePath string) (*rsa.PrivateKey, error) {
	pemData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "failed to read PEM file")
	}

	return ParsePrivateKeyFromPEM(pemData)
}

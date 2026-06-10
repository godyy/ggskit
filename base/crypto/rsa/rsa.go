package rsa

import (
	stdcrypto "crypto"
	"crypto/rand"
	"crypto/rsa"

	"github.com/godyy/ggskit/base/crypto"
	pkgerrors "github.com/pkg/errors"
)

// Encryptor 加密器：仅持有公钥用于加密
type Encryptor struct {
	pubKey *rsa.PublicKey
}

// PublicKey 返回加密器的公钥
func (e *Encryptor) PublicKey() *rsa.PublicKey { return e.pubKey }

// EncryptedLen 返回按 PKCS#1 v1.5 分块加密后的密文长度。
// 每块密文长度为 keySize，明文分块最大为 keySize-11。
func (e *Encryptor) EncryptedLen(dataLen int) int {
	keySize := e.pubKey.Size()
	if dataLen <= 0 || keySize <= 0 {
		return 0
	}
	maxChunkSize := keySize - 11
	if maxChunkSize <= 0 {
		return 0
	}
	chunks := (dataLen + maxChunkSize - 1) / maxChunkSize
	return chunks * keySize
}

// Encrypt 使用公钥加密数据（支持大数据分块加密）
func (e *Encryptor) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, pkgerrors.New("rsa: empty db")
	}

	keySize := e.pubKey.Size()
	maxChunkSize := keySize - 11
	if maxChunkSize <= 0 {
		return nil, pkgerrors.New("rsa: invalid key size")
	}

	total := e.EncryptedLen(len(data))
	result := make([]byte, total)
	offset := 0
	for i := 0; i < len(data); i += maxChunkSize {
		end := i + maxChunkSize
		if end > len(data) {
			end = len(data)
		}
		enc, err := rsa.EncryptPKCS1v15(rand.Reader, e.pubKey, data[i:end])
		if err != nil {
			return nil, pkgerrors.WithMessagef(err, "encrypt chunk [%d,%d)", i, end)
		}
		copy(result[offset:offset+len(enc)], enc)
		offset += len(enc)
	}
	return result, nil
}

// RSADecryptor 解密器：仅持有私钥用于解密
type RSADecryptor struct {
	priKey *rsa.PrivateKey
}

// PrivateKey 返回解密器的私钥
func (d *RSADecryptor) PrivateKey() *rsa.PrivateKey { return d.priKey }

// Decrypt 使用私钥解密数据（支持大数据分块解密）
func (d *RSADecryptor) Decrypt(data []byte) ([]byte, error) {
	keySize := d.priKey.Size()
	if len(data) <= keySize {
		return rsa.DecryptPKCS1v15(rand.Reader, d.priKey, data)
	}

	if len(data)%keySize != 0 {
		return nil, crypto.ErrEncryptedDataLength
	}

	var result []byte
	for i := 0; i < len(data); i += keySize {
		chunk := data[i : i+keySize]
		decryptedChunk, err := rsa.DecryptPKCS1v15(rand.Reader, d.priKey, chunk)
		if err != nil {
			return nil, pkgerrors.WithMessagef(err, "decrypt chunk [%d,%d)", i, i+keySize)
		}
		result = append(result, decryptedChunk...)
	}
	return result, nil
}

// RSASigner 仅用于签名（持有私钥）
type RSASigner struct {
	priKey *rsa.PrivateKey
	hash   stdcrypto.Hash
}

// PrivateKey 返回签名器的私钥
func (s *RSASigner) PrivateKey() *rsa.PrivateKey { return s.priKey }

// Hash 返回签名器使用的哈希函数
func (s *RSASigner) Hash() stdcrypto.Hash { return s.hash }

// Sign 使用私钥对数据进行签名
func (s *RSASigner) Sign(data []byte) ([]byte, error) {
	hashFunc := s.hash.New()
	hashFunc.Write(data)
	hashed := hashFunc.Sum(nil)
	return rsa.SignPSS(rand.Reader, s.priKey, s.hash, hashed, nil)
}

// RSAVerifier 仅用于验签（持有公钥）
type RSAVerifier struct {
	pubKey *rsa.PublicKey
	hash   stdcrypto.Hash
}

// PublicKey 返回验签器的公钥
func (v *RSAVerifier) PublicKey() *rsa.PublicKey { return v.pubKey }

// Hash 返回验签器使用的哈希函数
func (v *RSAVerifier) Hash() stdcrypto.Hash { return v.hash }

// Verify 使用公钥验证签名
func (v *RSAVerifier) Verify(data, signature []byte) error {
	hashFunc := v.hash.New()
	hashFunc.Write(data)
	hashed := hashFunc.Sum(nil)
	return rsa.VerifyPSS(v.pubKey, v.hash, hashed, signature, nil)
}

// NewRSAEncryptor 创建加密器
func NewRSAEncryptor(pubKey *rsa.PublicKey) *Encryptor {
	if pubKey == nil {
		panic("public key must not be nil")
	}
	return &Encryptor{
		pubKey: pubKey,
	}
}

// NewRSADecryptor 创建解密器
func NewRSADecryptor(priKey *rsa.PrivateKey) *RSADecryptor {
	if priKey == nil {
		panic("private key must not be nil")
	}
	return &RSADecryptor{
		priKey: priKey,
	}
}

// NewRSASigner 创建签名器
func NewRSASigner(priKey *rsa.PrivateKey, hash stdcrypto.Hash) *RSASigner {
	return &RSASigner{priKey: priKey, hash: hash}
}

// NewRSAVerifier 创建验签器
func NewRSAVerifier(pubKey *rsa.PublicKey, hash stdcrypto.Hash) *RSAVerifier {
	return &RSAVerifier{pubKey: pubKey, hash: hash}
}

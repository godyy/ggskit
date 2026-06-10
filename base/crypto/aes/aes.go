package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"

	"github.com/godyy/ggskit/base/crypto"
	pkgerrors "github.com/pkg/errors"
)

// ErrKeySize 密钥长度错误
var ErrKeySize = errors.New("key size error")

// Cryptor AES加密工具接口
type Cryptor interface {
	crypto.Encryptor
	crypto.Decryptor

	// Mode 获取加密模式
	Mode() string

	// Key 获取密钥
	Key() []byte
}

// ErrGCMNonceSize GCM nonce大小错误
var ErrGCMNonceSize = errors.New("GCM nonce size error")

// AESGCMCryptor AES-GCM对称加密器
type AESGCMCryptor struct {
	key   []byte
	nonce []byte
	gcm   cipher.AEAD
}

// EncryptedLen 返回加密后数据的长度
func (a *AESGCMCryptor) EncryptedLen(srcLen int) int {
	return srcLen + a.gcm.Overhead()
}

// Encrypt 加密数据
func (a *AESGCMCryptor) Encrypt(data []byte) ([]byte, error) {
	// 加密数据并直接返回密文
	encrypted := a.gcm.Seal(nil, a.nonce, data, nil)
	return encrypted, nil
}

// Decrypt 解密数据
func (a *AESGCMCryptor) Decrypt(data []byte) ([]byte, error) {
	// 检查密文长度
	if len(data) < a.gcm.Overhead() {
		return nil, crypto.ErrEncryptedDataLength
	}

	// 使用构造时设置的IV进行解密
	return a.gcm.Open(nil, a.nonce, data, nil)
}

// Key 获取密钥
func (a *AESGCMCryptor) Key() []byte {
	return a.key
}

// Mode 获取加密模式
func (a *AESGCMCryptor) Mode() string {
	return "GCM"
}

// ErrCBCIVSize CBC IV大小错误
var ErrCBCIVSize = errors.New("CBC IV size error")

// AESCBCCryptor AES-CBC对称加密器
type AESCBCCryptor struct {
	key       []byte
	encrypter cipher.BlockMode
	decrypter cipher.BlockMode
}

// EncryptedLen 返回加密后数据的长度
func (a *AESCBCCryptor) EncryptedLen(srcLen int) int {
	bs := aes.BlockSize
	return ((srcLen / bs) + 1) * bs
}

// Encrypt 加密数据
func (a *AESCBCCryptor) Encrypt(data []byte) ([]byte, error) {
	// PKCS7填充
	paddedData := pkcs7Pad(data, aes.BlockSize)

	// 使用预创建的CBC加密器
	encrypted := make([]byte, len(paddedData))
	a.encrypter.CryptBlocks(encrypted, paddedData)

	// 直接返回密文，不追加IV
	return encrypted, nil
}

// Decrypt 解密数据
func (a *AESCBCCryptor) Decrypt(data []byte) ([]byte, error) {
	// 检查密文长度是否为块大小的倍数
	if len(data)%aes.BlockSize != 0 {
		return nil, crypto.ErrEncryptedDataLength
	}

	// 使用预创建的CBC解密器
	decrypted := make([]byte, len(data))
	a.decrypter.CryptBlocks(decrypted, data)

	// 去除PKCS7填充
	return pkcs7Unpad(decrypted)
}

// Key 获取密钥
func (a *AESCBCCryptor) Key() []byte {
	return a.key
}

// Mode 获取加密模式
func (a *AESCBCCryptor) Mode() string {
	return "CBC"
}

// NewAESGCMCryptor 从现有密钥和nonce创建AES-GCM加密器
// nonce的长度必须为12.
func NewAESGCMCryptor(key []byte, nonce []byte) (*AESGCMCryptor, error) {
	// 验证密钥长度
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrKeySize
	}

	// 创建AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "create cipher failed")
	}

	// 创建GCM实例
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "create gcm failed")
	}

	// 检查nonce长度
	if len(nonce) != gcm.NonceSize() {
		return nil, ErrGCMNonceSize
	}

	// 创建AES-GCM加密器
	aes := &AESGCMCryptor{
		key:   key,
		nonce: nonce,
		gcm:   gcm,
	}
	return aes, nil
}

// NewAESCBCCryptoFromKeyWithIV 从现有密钥和可选IV创建AES-CBC加密器
// iv 的长度必须是 aes.BlockSize.
func NewAESCBCCryptor(key []byte, iv []byte) (*AESCBCCryptor, error) {
	// 验证密钥长度
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrKeySize
	}

	// 验证IV长度
	if len(iv) != aes.BlockSize {
		return nil, ErrCBCIVSize
	}

	// 创建AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "create cipher failed")
	}

	// 创建AES-CBC加密工具
	aes := &AESCBCCryptor{
		key:       key,
		encrypter: cipher.NewCBCEncrypter(block, iv),
		decrypter: cipher.NewCBCDecrypter(block, iv),
	}

	return aes, nil
}

package crypto

// Encryptor 加密器
type Encryptor interface {
	// EncryptedLen 返回加密后数据的长度
	EncryptedLen(dataLen int) int

	// Encrypt 加密数据
	Encrypt(data []byte) ([]byte, error)
}

// Decryptor 解密器
type Decryptor interface {
	// Decrypt 解密数据
	Decrypt(data []byte) ([]byte, error)
}

// Signer 签名器
type Signer interface {
	// Sign 签名数据
	Sign(data []byte) ([]byte, error)
}

// Verifier 验签器
type Verifier interface {
	// Verify 验签数据
	Verify(data []byte, signature []byte) error
}

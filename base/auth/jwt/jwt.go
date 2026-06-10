package jwt

import (
	"crypto"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	pkgerrors "github.com/pkg/errors"
	"github.com/rs/xid"
)

// LoadPrivKey 加载私钥.
func LoadPrivKey(path string) (crypto.PrivateKey, error) {
	priPem, err := os.ReadFile(path)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "read file")
	}
	priKey, err := jwt.ParseEdPrivateKeyFromPEM(priPem)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "parse private key")
	}
	return priKey, nil
}

// LoadPubKey 加载公钥.
func LoadPubKey(path string) (crypto.PublicKey, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "read file")
	}
	pubKey, err := jwt.ParseEdPublicKeyFromPEM(pemBytes)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "parse public key")
	}
	return pubKey, nil
}

// SignToken 签发JWT Token.
func SignToken(priKey crypto.PrivateKey, iss string, sub string, exp time.Duration, now time.Time) (string, error) {
	claims := make(jwt.MapClaims)
	claims["iss"] = iss
	claims["sub"] = sub
	claims["iat"] = now.Unix()
	claims["nbf"] = now.Unix()
	claims["exp"] = now.Add(exp).Unix()
	claims["jti"] = xid.New().String()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signedToken, err := token.SignedString(priKey)
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

// ParseToken 解析JWT Token.
func ParseToken(pubKey crypto.PublicKey, tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, pkgerrors.WithMessage(jwt.ErrSignatureInvalid, "unexpected signing method")
		}
		return pubKey, nil
	})
	if err != nil {
		return nil, err
	}
	return token.Claims.(jwt.MapClaims), nil
}

// GetSub 获取JWT Token中的sub字段.
func GetSub(claims jwt.MapClaims) (string, bool) {
	sub, ok := claims["sub"].(string)
	return sub, ok
}

package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"

	"github.com/xlvector/dlog"
)

func DecodePassword(p string, privateKey *rsa.PrivateKey) string {
	bp, err := hex.DecodeString(p)
	if err != nil {
		dlog.Warn("decode password hex error:%s", err.Error())
		return ""
	}
	out, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey,
		bp, []byte(""))
	if err != nil {
		dlog.Warn("decode password error:%s", err.Error())
		return ""
	}
	return string(out)
}

func GenerateRSAKey() (*rsa.PrivateKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}
	privKey.Precompute()
	err = privKey.Validate()
	if err != nil {
		return nil, err
	}
	return privKey, nil
}

func PublicKeyString(key *rsa.PublicKey) []byte {
	b, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil
	}
	pbs := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: b,
	})
	return pbs
}

func PrivateKeyString(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
}

func pkcs1pad2(s []byte, n int) (*big.Int, error) {
	if n < len(s)+11 {
		return nil, rsa.ErrMessageTooLong
	}
	ba := make([]byte, n)
	i := len(s) - 1
	for i >= 0 && n > 0 {
		c := s[i]
		i -= 1
		if c < 128 {
			n -= 1
			ba[n] = c
		} else if (c > 127) && (c < 255) {
			n -= 1
			ba[n] = (c & 63) | 128
			n -= 1
			ba[n] = (c >> 6) | 192
		} else {
			n -= 1
			ba[n] = (c & 63) | 128
			n -= 1
			ba[n] = ((c >> 6) & 63) | 128
			n -= 1
			ba[n] = (c >> 12) | 224
		}
	}
	n -= 1
	ba[n] = 0
	x := make([]byte, 1)
	for n > 2 {
		x[0] = byte(0)
		for x[0] == 0 {
			rand.Read(x)
		}
		n -= 1
		ba[n] = x[0]
	}
	n -= 1
	ba[n] = 2
	n -= 1
	ba[n] = 0
	ret := new(big.Int)
	return ret.SetBytes(ba), nil
}

func rsaDoPublic(x *big.Int, pub *rsa.PublicKey) *big.Int {
	e := big.NewInt(int64(pub.E))
	ret := new(big.Int)
	return ret.Exp(x, e, pub.N)
}

func PKCS1Pad2Encrypt(s []byte, pub *rsa.PublicKey) (string, error) {
	m, err := pkcs1pad2(s, int(pub.N.BitLen()+7)>>3)
	if err != nil {
		return "", err
	}
	c := rsaDoPublic(m, pub)
	if c == nil {
		return "", errors.New("do public failed")
	}
	h := hex.EncodeToString(c.Bytes())
	if len(h)&1 == 0 {
		return h, nil
	}
	return "0" + h, nil
}

func AESEncodePassword(pwd, key, iv string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		dlog.Warn("aes newCipher from key %s error", key)
		return "", err
	}
	if len(iv) == 0 {
		iv = "0000000000000000" //default iv
	}

	ecb := cipher.NewCBCEncrypter(block, []byte(iv))
	content := []byte(pwd)
	content = PKCS7Padding(content, block.BlockSize())
	crypted := make([]byte, len(content))
	ecb.CryptBlocks(crypted, content)
	return base64.StdEncoding.EncodeToString(crypted), nil
}

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

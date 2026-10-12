package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"math"
	"math/big"

	pbSecureMessage "github.com/ACLzz/qshare/internal/protobuf/gen/securemessage"
)

/*
	receiver -- site of communication which runs the code
	sender -- opposite site of communication
*/

const (
	authLabel   = "UKEY2 v1 auth"
	secretLabel = "UKEY2 v1 next"
)

//nolint:golines
var (
	d2dSalt = must(hex.DecodeString("82AA55A0D397F88346CA1CEE8D3909B95F13FA7DEB1D4AB38376B8256DA85510"))
	encSalt = must(hex.DecodeString("BF9D2A53C63616D75DB0A7165B91C1EF73E537F2427405FA23610A4BE657642E"))
)

type Cipher struct {
	isServer bool

	receiverInitMsg []byte
	senderInitMsg   []byte

	authPin            uint16
	senderHMAC         hash.Hash
	receiverHMAC       hash.Hash
	receiverPrivateKey *ecdh.PrivateKey
	senderPublicKey    *ecdh.PublicKey
	decryptBlock       cipher.Block
	encryptBlock       cipher.Block
}

func NewCipher(isServer bool) *Cipher {
	return &Cipher{
		isServer: isServer,
	}
}

func (c *Cipher) SetSenderInitMessage(msg []byte) error {
	if len(msg) == 0 {
		return ErrInvalidSenderInitMessage
	}

	c.senderInitMsg = msg
	return nil
}

func (c *Cipher) SetReceiverInitMessage(msg []byte) error {
	if len(msg) == 0 {
		return ErrInvalidReceiverInitMessage
	}

	c.receiverInitMsg = msg
	return nil
}

func (c *Cipher) SetReceiverPrivateKey(key *ecdsa.PrivateKey) (err error) {
	if key == nil {
		return ErrInvalidReceiverPrivateKey
	}

	c.receiverPrivateKey, err = key.ECDH()
	if err != nil {
		return fmt.Errorf("convert ecdsa private key to ecdh: %w", err)
	}
	return nil
}

func (c *Cipher) SetSenderPublicKey(key *pbSecureMessage.EcP256PublicKey) error {
	x := key.GetX()
	y := key.GetY()
	if len(x) > 32 {
		x = x[len(x)-32:]
	}
	if len(y) > 32 {
		y = y[len(y)-32:]
	}

	var err error
	c.senderPublicKey, err = (&ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     big.NewInt(0).SetBytes(x),
		Y:     big.NewInt(0).SetBytes(y),
	}).ECDH()
	if err != nil {
		return fmt.Errorf("craft ecdh public key: %w", err)
	}

	return nil
}

func (c *Cipher) validate() bool {
	return len(c.senderInitMsg) > 0 &&
		len(c.receiverInitMsg) > 0 &&
		c.receiverPrivateKey != nil &&
		c.senderPublicKey != nil
}

func (c *Cipher) craftD2DKeys() ([]byte, []byte, error) {
	// derive secret hash
	secret, err := c.receiverPrivateKey.ECDH(c.senderPublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("get dh secret: %w", err)
	}
	secretHash := sha256.Sum256(secret)

	// derive connection hash
	var senderInfo, receiverInfo string
	var ukeyInfo []byte
	if c.isServer {
		senderInfo, receiverInfo = "client", "server"
		ukeyInfo = make([]byte, len(c.receiverInitMsg)+len(c.senderInitMsg))
		copy(ukeyInfo, c.senderInitMsg)
		copy(ukeyInfo[len(c.senderInitMsg):], c.receiverInitMsg)
	} else {
		senderInfo, receiverInfo = "server", "client"
		ukeyInfo = make([]byte, len(c.receiverInitMsg)+len(c.senderInitMsg))
		copy(ukeyInfo, c.receiverInitMsg)
		copy(ukeyInfo[len(c.receiverInitMsg):], c.senderInitMsg)
	}

	connSecret, err := hkdfExtractExpand(secretHash[:], []byte(secretLabel), string(ukeyInfo))
	if err != nil {
		return nil, nil, fmt.Errorf("generate connection secret: %w", err)
	}

	// derive auth code
	authSecret, err := hkdfExtractExpand(secretHash[:], []byte(authLabel), string(ukeyInfo))
	if err != nil {
		return nil, nil, fmt.Errorf("generate authentication secret: %w", err)
	}
	c.authPin = authSecretToPin(authSecret)

	// derive device to device keys
	d2dSenderKey, err := hkdfExtractExpand(connSecret, d2dSalt, senderInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("derive d2d sender key: %w", err)
	}

	d2dReceiverKey, err := hkdfExtractExpand(connSecret, d2dSalt, receiverInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("derive d2d receiver key: %w", err)
	}

	return d2dReceiverKey, d2dSenderKey, nil
}

func (c *Cipher) Setup() error {
	if !c.validate() {
		return ErrInvalidCipher
	}

	// craft device to device keys
	d2dReceiverKey, d2dSenderKey, err := c.craftD2DKeys()
	if err != nil {
		return fmt.Errorf("craft d2d keys: %w", err)
	}

	// derive HMAC keys
	senderHMACKey, err := hkdfExtractExpand(d2dSenderKey, encSalt, "SIG:1")
	if err != nil {
		return fmt.Errorf("derive sender hmac key: %w", err)
	}

	receiverHMACKey, err := hkdfExtractExpand(d2dReceiverKey, encSalt, "SIG:1")
	if err != nil {
		return fmt.Errorf("derive receiver hmac key: %w", err)
	}

	c.senderHMAC = hmac.New(sha256.New, senderHMACKey)
	c.receiverHMAC = hmac.New(sha256.New, receiverHMACKey)

	// derive keys and create decrypt/encrypt blocks
	decryptKey, err := hkdfExtractExpand(d2dSenderKey, encSalt, "ENC:2")
	if err != nil {
		return fmt.Errorf("derive decrypt key: %w", err)
	}

	encryptKey, err := hkdfExtractExpand(d2dReceiverKey, encSalt, "ENC:2")
	if err != nil {
		return fmt.Errorf("derive encrypt key: %w", err)
	}

	c.decryptBlock, err = aes.NewCipher(decryptKey)
	if err != nil {
		return fmt.Errorf("create decrypt block: %w", err)
	}

	c.encryptBlock, err = aes.NewCipher(encryptKey)
	if err != nil {
		return fmt.Errorf("create encrypt block: %w", err)
	}

	return nil
}

func (c *Cipher) ValidateSignature(hb, signature []byte) error {
	defer c.senderHMAC.Reset()
	if _, err := c.senderHMAC.Write(hb); err != nil {
		return fmt.Errorf("get hmac_sha256 from header and body: %w", err)
	}

	expectedSig := c.senderHMAC.Sum(nil)
	for i := range expectedSig {
		if expectedSig[i] != signature[i] {
			return ErrInvalidSecureMessageSignature
		}
	}

	return nil
}

func (c *Cipher) Sign(data []byte) []byte {
	defer c.receiverHMAC.Reset()
	c.receiverHMAC.Write(data)
	return c.receiverHMAC.Sum(nil)
}

func (c *Cipher) Pin() uint16 {
	return c.authPin
}

func hkdfExtractExpand(key, salt []byte, info string) ([]byte, error) {
	extKey, err := hkdf.Extract(sha256.New, key, salt)
	if err != nil {
		return nil, fmt.Errorf("extract hkdf key: %w", err)
	}

	exp, err := hkdf.Expand(sha256.New, extKey, info, 32)
	if err != nil {
		return nil, fmt.Errorf("expand hkdf key: %w", err)
	}

	return exp, nil
}

func must[T any](retValue T, err error) T {
	if err != nil {
		panic(err)
	}

	return retValue
}

const (
	hash_mod       = 9973
	hash_base_mult = 31
)

func authSecretToPin(secret []byte) uint16 {
	hash := 0
	mult := 1
	for _, b := range secret {
		hash = (hash + int(int8(b))*mult) % hash_mod
		mult = (mult * hash_base_mult) % hash_mod
	}

	return uint16(math.Abs(float64(hash)))
}

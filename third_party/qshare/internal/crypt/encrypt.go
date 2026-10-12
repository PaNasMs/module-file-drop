package crypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	pbSecuregcm "github.com/ACLzz/qshare/internal/protobuf/gen/securegcm"
	pbSecureMessage "github.com/ACLzz/qshare/internal/protobuf/gen/securemessage"
	"google.golang.org/protobuf/proto"
)

func (c *Cipher) Encrypt(
	d2dMsg *pbSecuregcm.DeviceToDeviceMessage,
) (*pbSecureMessage.HeaderAndBody, error) {
	msg, err := proto.Marshal(d2dMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal device to device message: %w", err)
	}

	msg = pad(msg)
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("generate random iv: %w", err)
	}

	enc := cipher.NewCBCEncrypter(c.encryptBlock, iv)
	enc.CryptBlocks(msg, msg)

	meta, err := proto.Marshal(&pbSecuregcm.GcmMetadata{
		Type:    pbSecuregcm.Type_DEVICE_TO_DEVICE_MESSAGE.Enum(),
		Version: proto.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	return &pbSecureMessage.HeaderAndBody{
		Header: &pbSecureMessage.Header{
			SignatureScheme:  pbSecureMessage.SigScheme_HMAC_SHA256.Enum(),
			EncryptionScheme: pbSecureMessage.EncScheme_AES_256_CBC.Enum(),
			Iv:               iv,
			PublicMetadata:   meta,
		},
		Body: msg,
	}, nil
}

func pad(body []byte) []byte {
	padLen := aes.BlockSize - len(body)%aes.BlockSize
	if padLen > 0 {
		padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
		body = append(body, padding...)
	}

	return body
}

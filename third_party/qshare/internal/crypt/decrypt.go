package crypt

import (
	"crypto/cipher"
	"fmt"

	pbSecuregcm "github.com/ACLzz/qshare/internal/protobuf/gen/securegcm"
	pbSecureMessage "github.com/ACLzz/qshare/internal/protobuf/gen/securemessage"
	"google.golang.org/protobuf/proto"
)

func (c *Cipher) Decrypt(
	hb *pbSecureMessage.HeaderAndBody,
) (*pbSecuregcm.DeviceToDeviceMessage, error) {
	body := make([]byte, len(hb.GetBody()))
	dec := cipher.NewCBCDecrypter(c.decryptBlock, hb.GetHeader().GetIv())
	dec.CryptBlocks(body, hb.GetBody())
	body = unpad(body)

	var d2dMsg pbSecuregcm.DeviceToDeviceMessage
	if err := proto.Unmarshal(body, &d2dMsg); err != nil {
		return nil, fmt.Errorf("unmarshal device to device message: %w", err)
	}

	return &d2dMsg, nil
}

// unpad: un-padding pcks7 padding
func unpad(body []byte) []byte {
	addedChars := int(body[len(body)-1])
	return body[:len(body)-addedChars]
}

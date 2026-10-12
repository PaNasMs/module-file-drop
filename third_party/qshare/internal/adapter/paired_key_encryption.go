package adapter

import (
	pbSharing "github.com/ACLzz/qshare/internal/protobuf/gen/sharing"
	"github.com/ACLzz/qshare/internal/rand"
)

func (a *Adapter) ValidatePairedKeyEncryption(msg []byte) error {
	frame, err := unmarshalSharingFrame(msg)
	if err != nil {
		return err
	}

	if frame.GetType() != pbSharing.V1Frame_PAIRED_KEY_ENCRYPTION ||
		frame.GetPairedKeyEncryption() == nil ||
		len(frame.PairedKeyEncryption.GetSignedData()) == 0 {
		return ErrInvalidMessage
	}

	return nil
}

func (a *Adapter) SendPairedKeyEncryption() error {
	return a.writeSecureFrame(&pbSharing.V1Frame{
		Type: pbSharing.V1Frame_PAIRED_KEY_ENCRYPTION.Enum(),
		PairedKeyEncryption: &pbSharing.PairedKeyEncryptionFrame{
			SecretIdHash: rand.Bytes(a.rand, 6),
			SignedData:   rand.Bytes(a.rand, 72),
		},
	})
}

package adapter

import (
	pbSharing "github.com/ACLzz/qshare/internal/protobuf/gen/sharing"
)

func (a *Adapter) ValidatePairedKeyResult(msg []byte) error {
	frame, err := unmarshalSharingFrame(msg)
	if err != nil {
		return err
	}

	if frame.GetType() != pbSharing.V1Frame_PAIRED_KEY_RESULT ||
		frame.GetPairedKeyResult() == nil ||
		frame.GetPairedKeyResult().GetStatus() != pbSharing.PairedKeyResultFrame_UNABLE {
		return ErrInvalidMessage
	}

	return nil
}

func (a *Adapter) SendPairedKeyResult() error {
	return a.writeSecureFrame(&pbSharing.V1Frame{
		Type: pbSharing.V1Frame_PAIRED_KEY_RESULT.Enum(),
		PairedKeyResult: &pbSharing.PairedKeyResultFrame{
			Status: pbSharing.PairedKeyResultFrame_UNABLE.Enum(),
		},
	})
}

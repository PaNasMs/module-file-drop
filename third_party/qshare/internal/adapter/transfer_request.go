package adapter

import pbSharing "github.com/ACLzz/qshare/internal/protobuf/gen/sharing"

func (a *Adapter) ValidateTransferRequest(msg []byte) error {
	frame, err := unmarshalSharingFrame(msg)
	if err != nil {
		return err
	}

	if frame.GetType() != pbSharing.V1Frame_RESPONSE ||
		frame.GetConnectionResponse() == nil ||
		frame.GetConnectionResponse().GetStatus() != pbSharing.ConnectionResponseFrame_ACCEPT {
		return ErrInvalidMessage
	}

	return nil
}

func (a *Adapter) SendTransferRequest() error {
	return a.writeSecureFrame(&pbSharing.V1Frame{
		Type: pbSharing.V1Frame_RESPONSE.Enum(),
		ConnectionResponse: &pbSharing.ConnectionResponseFrame{
			Status: pbSharing.ConnectionResponseFrame_ACCEPT.Enum(),
		},
	})
}

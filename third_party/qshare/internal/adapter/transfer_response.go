package adapter

import (
	pbSharing "github.com/ACLzz/qshare/internal/protobuf/gen/sharing"
)

func (a *Adapter) UnmarshalTransferResponse(msg []byte) (bool, error) {
	frame, err := unmarshalSharingFrame(msg)
	if err != nil {
		return false, err
	}

	if frame.GetType() != pbSharing.V1Frame_RESPONSE || frame.GetConnectionResponse() == nil {
		return false, ErrInvalidMessage
	}

	status := frame.GetConnectionResponse().GetStatus()
	return status == pbSharing.ConnectionResponseFrame_ACCEPT, nil
}

func (a *Adapter) SendTransferResponse(isAccepted bool) error {
	status := pbSharing.ConnectionResponseFrame_REJECT
	if isAccepted {
		status = pbSharing.ConnectionResponseFrame_ACCEPT
	}

	return a.writeSecureFrame(&pbSharing.V1Frame{
		Type: pbSharing.V1Frame_RESPONSE.Enum(),
		ConnectionResponse: &pbSharing.ConnectionResponseFrame{
			Status: status.Enum(),
		},
	})
}

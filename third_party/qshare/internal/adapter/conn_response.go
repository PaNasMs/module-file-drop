package adapter

import (
	"runtime"

	pbConnections "github.com/ACLzz/qshare/internal/protobuf/gen/connections"
)

type ConnResponse struct {
	IsConnAccepted bool
}

func (a *Adapter) UnmarshalConnResponse(msg []byte) (ConnResponse, error) {
	frame, err := unmarshalOfflineFrame(msg)
	if err != nil {
		return ConnResponse{}, err
	}

	if frame.GetType() != pbConnections.V1Frame_CONNECTION_RESPONSE ||
		frame.GetConnectionResponse() == nil {
		return ConnResponse{}, ErrInvalidMessage
	}

	response := frame.GetConnectionResponse().GetResponse()
	return ConnResponse{
		IsConnAccepted: response == pbConnections.ConnectionResponseFrame_ACCEPT,
	}, nil
}

func (a *Adapter) SendConnResponse(isAccepted bool) error {
	osType := pbConnections.OsInfo_LINUX
	switch runtime.GOOS {
	case "windows":
		osType = pbConnections.OsInfo_WINDOWS
	case "android":
		osType = pbConnections.OsInfo_ANDROID
	case "ios":
		osType = pbConnections.OsInfo_APPLE
	}

	response := pbConnections.ConnectionResponseFrame_ACCEPT
	if !isAccepted {
		response = pbConnections.ConnectionResponseFrame_REJECT
	}

	return a.writeOfflineFrame(&pbConnections.V1Frame{
		Type: pbConnections.V1Frame_CONNECTION_RESPONSE.Enum(),
		ConnectionResponse: &pbConnections.ConnectionResponseFrame{
			Response: response.Enum(),
			OsInfo: &pbConnections.OsInfo{
				Type: osType.Enum(),
			},
		},
	})
}

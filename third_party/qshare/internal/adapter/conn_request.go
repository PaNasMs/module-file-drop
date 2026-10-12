package adapter

import (
	"github.com/ACLzz/qshare"
	pbConnections "github.com/ACLzz/qshare/internal/protobuf/gen/connections"
	"github.com/ACLzz/qshare/internal/rand"
	"google.golang.org/protobuf/proto"
)

type ConnRequest struct {
	EndpointInfo []byte
}

func (a *Adapter) UnmarshalConnRequest(msg []byte) (ConnRequest, error) {
	frame, err := unmarshalOfflineFrame(msg)
	if err != nil {
		return ConnRequest{}, err
	}

	if frame.GetType() != pbConnections.V1Frame_CONNECTION_REQUEST ||
		frame.GetConnectionRequest() == nil {
		return ConnRequest{}, ErrInvalidMessage
	}

	return ConnRequest{
		EndpointInfo: frame.GetConnectionRequest().GetEndpointInfo(),
	}, nil
}

func (a *Adapter) SendConnRequest(endpointID, hostname string, device qshare.DeviceType) error {
	return a.writeOfflineFrame(&pbConnections.V1Frame{
		Type: pbConnections.V1Frame_CONNECTION_REQUEST.Enum(),
		ConnectionRequest: &pbConnections.ConnectionRequestFrame{
			EndpointId:   proto.String(endpointID),
			EndpointName: []byte(hostname),
			EndpointInfo: CraftEndpointInfo(a.rand, hostname, device),
			Mediums: []pbConnections.ConnectionRequestFrame_Medium{
				pbConnections.ConnectionRequestFrame_WIFI_LAN,
			},
		},
	})
}

func CraftEndpointInfo(r rand.Random, hostname string, device qshare.DeviceType) []byte {
	hostnameBytes := []byte(hostname)
	buf := make([]byte, 1, len(hostnameBytes)+17)

	buf[0] = byte(device << 1)

	buf = append(buf, rand.Bytes(r, 16)...)
	buf = append(buf, byte(len(hostnameBytes)))
	buf = append(buf, hostnameBytes...)

	return buf
}

package adapter

import (
	pbConnections "github.com/ACLzz/qshare/internal/protobuf/gen/connections"
	pbSecuregcm "github.com/ACLzz/qshare/internal/protobuf/gen/securegcm"
	"google.golang.org/protobuf/proto"
)

func (a *Adapter) ProcessServiceMessage(msg []byte) error {
	var (
		offFrame         pbConnections.OfflineFrame
		ukeyMessage      pbSecuregcm.Ukey2Message
		isServiceMessage bool
	)

	if offErr := proto.Unmarshal(msg, &offFrame); offErr == nil {
		if offFrame.GetV1().GetType() == pbConnections.V1Frame_KEEP_ALIVE {
			isServiceMessage = true
			a.sendKeepAliveMessage()
		}
		if offFrame.GetV1().GetType() == pbConnections.V1Frame_DISCONNECTION {
			return ErrConnWasEndedByClient
		}
	} else if ukeyErr := proto.Unmarshal(msg, &ukeyMessage); ukeyErr == nil {
		if ukeyMessage.GetMessageType() == pbSecuregcm.Ukey2Message_ALERT {
			isServiceMessage = true
			a.logUKEYAlert(&ukeyMessage)
		}
	}

	if !isServiceMessage {
		return ErrNotServiceMessage
	}

	return nil
}

func (a *Adapter) sendKeepAliveMessage() {
	keepAliveFrame := pbConnections.V1Frame{
		Type: pbConnections.V1Frame_KEEP_ALIVE.Enum(),
		KeepAlive: &pbConnections.KeepAliveFrame{
			Ack: proto.Bool(true),
		},
	}

	var err error
	if a.isEncrypted {
		err = a.encryptAndWrite(&keepAliveFrame)
	} else {
		err = a.writeOfflineFrame(&keepAliveFrame)
	}
	if err != nil {
		a.log.Error("send keep alive message", err)
		return
	}

	a.log.Debug("sent keep alive message")
}

func (a *Adapter) logUKEYAlert(ukeyMessage *pbSecuregcm.Ukey2Message) {
	var alert pbSecuregcm.Ukey2Alert
	if err := proto.Unmarshal(ukeyMessage.GetMessageData(), &alert); err != nil {
		return
	}

	alertType, ok := pbSecuregcm.Ukey2Alert_AlertType_name[int32(alert.GetType())]
	if !ok {
		return
	}

	a.log.Warn("got an alert", "type", alertType, "message", alert.GetErrorMessage())
}

package adapter

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"

	"github.com/ACLzz/qshare"
	pbConnections "github.com/ACLzz/qshare/internal/protobuf/gen/connections"
	pbSecuregcm "github.com/ACLzz/qshare/internal/protobuf/gen/securegcm"
	pbSecureMessage "github.com/ACLzz/qshare/internal/protobuf/gen/securemessage"
	pbSharing "github.com/ACLzz/qshare/internal/protobuf/gen/sharing"
	"google.golang.org/protobuf/proto"
)

const FILE_CHUNK_SIZE = 512 * 1024 // 512 KB

func (a *Adapter) writeMessage(data []byte) error {
	msgLength := uint32(len(data))
	msgWithPrefix := make([]byte, msgLength+4)

	binary.BigEndian.PutUint32(msgWithPrefix[:4], msgLength)
	copy(msgWithPrefix[4:], data)
	if _, err := a.conn.Write(msgWithPrefix); err != nil {
		return fmt.Errorf("send message to client: %w", err)
	}

	return nil
}

func (a *Adapter) writeOfflineFrame(frame *pbConnections.V1Frame) error {
	if a.isEncrypted {
		err := a.encryptAndWrite(frame)
		if err != nil {
			return err
		}
	} else {
		offlineFrame, err := proto.Marshal(&pbConnections.OfflineFrame{
			Version: pbConnections.OfflineFrame_V1.Enum(),
			V1:      frame,
		})
		if err != nil {
			return err
		}
		if err = a.writeMessage(offlineFrame); err != nil {
			return fmt.Errorf("write message: %w", err)
		}
	}

	return nil
}

func (a *Adapter) writeUKEYMessage(t pbSecuregcm.Ukey2Message_Type, msg []byte) error {
	ukeyMsg, err := proto.Marshal(&pbSecuregcm.Ukey2Message{
		MessageType: t.Enum(),
		MessageData: msg,
	})
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	if err = a.writeMessage(ukeyMsg); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	return nil
}

func (a *Adapter) writeSecureFrame(frame *pbSharing.V1Frame) error {
	data, err := proto.Marshal(&pbSharing.Frame{
		Version: pbSharing.Frame_V1.Enum(),
		V1:      frame,
	})
	if err != nil {
		return fmt.Errorf("marshal secure frame: %w", err)
	}

	return a.SendDataInChunks(rand.Int64(), data)
}

func (a *Adapter) encryptAndWrite(frame *pbConnections.V1Frame) error {
	offlineFrame, err := proto.Marshal(&pbConnections.OfflineFrame{
		Version: pbConnections.OfflineFrame_V1.Enum(),
		V1:      frame,
	})
	if err != nil {
		return fmt.Errorf("marshal offline frame: %w", err)
	}

	a.seqNumber++
	hb, err := a.cipher.Encrypt(&pbSecuregcm.DeviceToDeviceMessage{
		SequenceNumber: proto.Int32(a.seqNumber),
		Message:        offlineFrame,
	})
	if err != nil {
		return fmt.Errorf("encrypt data: %w", err)
	}

	data, err := proto.Marshal(hb)
	if err != nil {
		return fmt.Errorf("marshal header and body: %w", err)
	}

	data, err = proto.Marshal(&pbSecureMessage.SecureMessage{
		HeaderAndBody: data,
		Signature:     a.cipher.Sign(data),
	})
	if err != nil {
		return fmt.Errorf("marshal secure message: %w", err)
	}

	if err := a.writeMessage(data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	return nil
}

func (a *Adapter) SendBadMessageError() {
	alertMsg, err := proto.Marshal(&pbSecuregcm.Ukey2Alert{
		Type:         pbSecuregcm.Ukey2Alert_BAD_MESSAGE.Enum(),
		ErrorMessage: proto.String(ErrInvalidMessage.Error()),
	})
	if err != nil {
		a.log.Error("marshal alert message", err)
		return
	}

	if err := a.writeUKEYMessage(pbSecuregcm.Ukey2Message_ALERT, alertMsg); err != nil {
		a.log.Error("send bad message error", err)
		return
	}
}

func (a *Adapter) SendDataInChunks(payloadID int64, data []byte) error {
	// TODO: make a limit per chunk
	// message
	if err := a.encryptAndWrite(&pbConnections.V1Frame{
		Type: pbConnections.V1Frame_PAYLOAD_TRANSFER.Enum(),
		PayloadTransfer: &pbConnections.PayloadTransferFrame{
			PacketType: pbConnections.PayloadTransferFrame_DATA.Enum(),
			PayloadChunk: &pbConnections.PayloadTransferFrame_PayloadChunk{
				Offset: proto.Int64(0),
				Flags:  proto.Int32(0),
				Body:   data,
			},
			PayloadHeader: &pbConnections.PayloadTransferFrame_PayloadHeader{
				Id:          proto.Int64(payloadID),
				Type:        pbConnections.PayloadTransferFrame_PayloadHeader_BYTES.Enum(),
				TotalSize:   proto.Int64(int64(len(data))),
				IsSensitive: proto.Bool(false),
			},
		},
	}); err != nil {
		return fmt.Errorf("encrypt and write message: %w", err)
	}

	// last chunk
	if err := a.encryptAndWrite(&pbConnections.V1Frame{
		Type: pbConnections.V1Frame_PAYLOAD_TRANSFER.Enum(),
		PayloadTransfer: &pbConnections.PayloadTransferFrame{
			PacketType: pbConnections.PayloadTransferFrame_DATA.Enum(),
			PayloadChunk: &pbConnections.PayloadTransferFrame_PayloadChunk{
				Offset: proto.Int64(int64(len(data))),
				Flags:  proto.Int32(1),
				Body:   []byte{},
			},
			PayloadHeader: &pbConnections.PayloadTransferFrame_PayloadHeader{
				Id:          proto.Int64(payloadID),
				Type:        pbConnections.PayloadTransferFrame_PayloadHeader_BYTES.Enum(),
				TotalSize:   proto.Int64(int64(len(data))),
				IsSensitive: proto.Bool(false),
			},
		},
	}); err != nil {
		return fmt.Errorf("encrypt and write last chunk: %w", err)
	}

	return nil
}

func (a *Adapter) SendFileInChunks(payloadID int64, file qshare.FilePayload) error {
	var (
		err    error
		n      int
		offset int64
	)
	defer func() {
		if err := file.Pr.Close(); err != nil {
			a.log.Error("close file pipe reader", err)
		}
	}()

	bufSize := FILE_CHUNK_SIZE
	if file.Meta.Size < FILE_CHUNK_SIZE {
		bufSize = int(file.Meta.Size)
	}
	buf := make([]byte, bufSize)

	// send chunks
	for {
		n, err = io.ReadFull(file.Pr, buf)
		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return fmt.Errorf("read file chunk: %w", err)
		}

		if err = a.encryptAndWrite(&pbConnections.V1Frame{
			Type: pbConnections.V1Frame_PAYLOAD_TRANSFER.Enum(),
			PayloadTransfer: &pbConnections.PayloadTransferFrame{
				PacketType: pbConnections.PayloadTransferFrame_DATA.Enum(),
				PayloadChunk: &pbConnections.PayloadTransferFrame_PayloadChunk{
					Offset: proto.Int64(offset),
					Flags:  proto.Int32(0),
					Body:   buf,
				},
				PayloadHeader: &pbConnections.PayloadTransferFrame_PayloadHeader{
					Id:          proto.Int64(payloadID),
					Type:        pbConnections.PayloadTransferFrame_PayloadHeader_FILE.Enum(),
					TotalSize:   proto.Int64(file.Meta.Size),
					FileName:    proto.String(file.Meta.Name),
					IsSensitive: proto.Bool(false),
				},
			},
		}); err != nil {
			return fmt.Errorf("encrypt and write file chunk: %w", err)
		}

		offset += int64(n)
		if n < FILE_CHUNK_SIZE {
			break
		}
	}

	// last chunk
	if err := a.encryptAndWrite(&pbConnections.V1Frame{
		Type: pbConnections.V1Frame_PAYLOAD_TRANSFER.Enum(),
		PayloadTransfer: &pbConnections.PayloadTransferFrame{
			PacketType: pbConnections.PayloadTransferFrame_DATA.Enum(),
			PayloadChunk: &pbConnections.PayloadTransferFrame_PayloadChunk{
				Offset: proto.Int64(offset),
				Flags:  proto.Int32(1),
				Body:   []byte{},
			},
			PayloadHeader: &pbConnections.PayloadTransferFrame_PayloadHeader{
				Id:          proto.Int64(payloadID),
				Type:        pbConnections.PayloadTransferFrame_PayloadHeader_FILE.Enum(),
				TotalSize:   proto.Int64(file.Meta.Size),
				FileName:    proto.String(file.Meta.Name),
				IsSensitive: proto.Bool(false),
			},
		},
	}); err != nil {
		return fmt.Errorf("encrypt and write last chunk: %w", err)
	}

	return nil
}

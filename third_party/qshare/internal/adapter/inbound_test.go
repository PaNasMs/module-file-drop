package adapter

import (
	"bytes"
	"testing"

	pb "github.com/ACLzz/qshare/internal/protobuf/gen/connections"
	"google.golang.org/protobuf/proto"
)

func bytesPayload(t *testing.T, id int64, body []byte) []byte {
	t.Helper()
	packet, err := proto.Marshal(&pb.OfflineFrame{
		Version: pb.OfflineFrame_V1.Enum(),
		V1: &pb.V1Frame{
			Type: pb.V1Frame_PAYLOAD_TRANSFER.Enum(),
			PayloadTransfer: &pb.PayloadTransferFrame{
				PacketType: pb.PayloadTransferFrame_DATA.Enum(),
				PayloadHeader: &pb.PayloadTransferFrame_PayloadHeader{
					Id: proto.Int64(id), Type: pb.PayloadTransferFrame_PayloadHeader_BYTES.Enum(), TotalSize: proto.Int64(int64(len(body))),
				},
				PayloadChunk: &pb.PayloadTransferFrame_PayloadChunk{
					Offset: proto.Int64(0), Flags: proto.Int32(1), Body: body,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

func TestTransferHandlerOnlyConsumesAdvertisedTextPayload(t *testing.T) {
	var texts []string
	a := &Adapter{textTransferCallback: func(value string) error {
		texts = append(texts, value)
		return nil
	}}
	a.EnableTransferHandler(nil)
	handle := a.transferHandler()
	control := []byte("control frame")
	got, err := handle(bytesPayload(t, 41, control))
	if err != nil || !bytes.Equal(got, control) || len(texts) != 0 {
		t.Fatalf("file-only transfer consumed control bytes: %q, %v, %v", got, err, texts)
	}

	a.EnableTransferHandler(NewTextMeta(7, 0, "", 4))
	got, err = handle(bytesPayload(t, 42, control))
	if err != nil || !bytes.Equal(got, control) || len(texts) != 0 {
		t.Fatalf("unannounced payload was consumed as text: %q, %v, %v", got, err, texts)
	}
	got, err = handle(bytesPayload(t, 7, []byte("note")))
	if err != nil || got != nil || len(texts) != 1 || texts[0] != "note" {
		t.Fatalf("advertised text payload was lost: %q, %v, %v", got, err, texts)
	}
}

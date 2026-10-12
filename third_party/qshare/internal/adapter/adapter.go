package adapter

import (
	"net"

	"github.com/ACLzz/qshare"
	"github.com/ACLzz/qshare/internal/crypt"
	pbConnections "github.com/ACLzz/qshare/internal/protobuf/gen/connections"
	"github.com/ACLzz/qshare/internal/rand"
)

type Adapter struct {
	conn   net.Conn
	cipher *crypt.Cipher
	log    qshare.Logger
	rand   rand.Random

	seqNumber            int32
	isEncrypted          bool
	isTransfer           bool
	textPayloadID        int64
	fileChunkCallback    FileChunkCallback
	textTransferCallback TextTransferCallback
}

func New(
	conn net.Conn,
	logger qshare.Logger,
	isServer bool,
	fileChunkCallback FileChunkCallback,
	textTransferCallback TextTransferCallback,
	r rand.Random,
) Adapter {
	return Adapter{
		conn:                 conn,
		cipher:               crypt.NewCipher(isServer),
		log:                  logger,
		fileChunkCallback:    fileChunkCallback,
		textTransferCallback: textTransferCallback,
		rand:                 r,
	}
}

func (a *Adapter) EnableEncryption() error {
	if err := a.cipher.Setup(); err != nil {
		return err
	}

	a.isEncrypted = true
	return nil
}

func (a *Adapter) Pin() uint16 {
	if !a.isEncrypted {
		return 0
	}

	return a.cipher.Pin()
}

func (a *Adapter) EnableTransferHandler(text *TextMeta) {
	a.isTransfer = text != nil
	if text != nil {
		a.textPayloadID = text.ID
	}
}

func (a *Adapter) Disconnect() {
	if err := a.writeOfflineFrame(&pbConnections.V1Frame{
		Type:          pbConnections.V1Frame_DISCONNECTION.Enum(),
		Disconnection: &pbConnections.DisconnectionFrame{},
	}); err != nil {
		a.log.Error("error while disconnecting", err)
	}
}

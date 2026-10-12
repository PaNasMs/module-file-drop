package listener

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ACLzz/qshare"
	adapter "github.com/ACLzz/qshare/internal/adapter"
	"github.com/ACLzz/qshare/internal/rand"
)

type filePayload struct {
	Pd         qshare.FilePayload
	IsNotified bool
	Complete   bool
}

// connection is a unit that communicate with client and routes it's messages.
// It is the core of the Server and all it's main logic is contained here.
type connection struct {
	ctx       context.Context
	cancelCtx context.CancelFunc
	adapter   adapter.Adapter
	conn      net.Conn
	log       qshare.Logger

	nextExpectedMessage expectedMessage
	phase               phase

	textMeta         *adapter.TextMeta // text payload can be sent only one
	filePayloads     map[int64]*filePayload
	expectedPayloads int
	receivedPayloads int
	authCallback     qshare.AuthCallback
	textCallback     qshare.TextCallback
	fileCallback     qshare.FileCallback
	endCallback      qshare.EndCallback
	approved         bool
}

func newConnection(
	ctx context.Context,
	conn net.Conn,
	logger qshare.Logger,
	authCallback qshare.AuthCallback,
	textCallback qshare.TextCallback,
	fileCallback qshare.FileCallback,
	endCallback qshare.EndCallback,
	r rand.Random,
) *connection {
	connCtx, cancel := context.WithCancel(ctx)
	c := &connection{
		ctx:                 connCtx,
		cancelCtx:           cancel,
		conn:                conn,
		log:                 logger,
		nextExpectedMessage: conn_request,
		phase:               init_phase,
		authCallback:        authCallback,
		textCallback:        textCallback,
		fileCallback:        fileCallback,
		endCallback:         endCallback,
	}
	c.adapter = adapter.New(conn, logger, true, c.writeFileChunk, c.writeText, r)

	return c
}

func (c *connection) Accept(wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if c.approved && c.endCallback != nil {
			c.endCallback(c.receivedPayloads == c.expectedPayloads)
		}
	}()
	defer c.cancelCtx()
	defer c.conn.Close()
	defer func() {
		for _, payload := range c.filePayloads {
			_ = payload.Pd.Pw.CloseWithError(io.ErrUnexpectedEOF)
		}
	}()

	c.log.Debug("got new connection")
	defer c.log.Debug("connection closed")

	readMsg := c.adapter.Reader(c.ctx)
	for {
		if err := c.conn.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return
		}
		// read the message
		msg, err := readMsg()
		if err != nil {
			if errors.Is(err, adapter.ErrOffsetMismatch) {
				c.adapter.SendBadMessageError()
			}
			if !errors.Is(err, io.EOF) && !errors.Is(err, adapter.ErrConnWasEndedByClient) {
				c.log.Error("read message", err)
			}
			return
		}

		// route the message
		c.route(msg)
	}
}

package listener

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/ACLzz/qshare"
	"github.com/ACLzz/qshare/internal/rand"
)

// Listener creates and manages all incoming connections.
type Listener struct {
	sock         net.Listener
	wg           *sync.WaitGroup
	log          qshare.Logger
	authCallback qshare.AuthCallback
	textCallback qshare.TextCallback
	fileCallback qshare.FileCallback
	endCallback  qshare.EndCallback
	stopCh       chan struct{}
	limit        chan struct{}
	active       *sync.Map
	rand         rand.Random
}

func New(
	port int,
	log qshare.Logger,
	authCallback qshare.AuthCallback,
	textCallback qshare.TextCallback,
	fileCallback qshare.FileCallback,
	endCallback qshare.EndCallback,
	r rand.Random,
) (Listener, error) {
	sock, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return Listener{}, fmt.Errorf("start socket: %w", err)
	}

	return Listener{
		sock:         sock,
		wg:           &sync.WaitGroup{},
		log:          log,
		authCallback: authCallback,
		textCallback: textCallback,
		fileCallback: fileCallback,
		endCallback:  endCallback,
		stopCh:       make(chan struct{}, 1),
		limit:        make(chan struct{}, 8),
		active:       &sync.Map{},
		rand:         r,
	}, nil
}

func (l Listener) Listen() {
	var (
		conn net.Conn
		err  error
	)
	l.wg.Add(1)
	defer l.wg.Done()
	l.log.Debug("listener started", "addr", l.sock.Addr().String())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		conn, err = l.sock.Accept()

		select {
		case <-l.stopCh:
			return
		default:
			if err != nil {
				l.log.Error("accept conn", err)
				continue
			}

			select {
			case l.limit <- struct{}{}:
			default:
				_ = conn.Close()
				continue
			}
			l.wg.Add(1)
			l.active.Store(conn, struct{}{})
			c := newConnection(
				ctx,
				conn,
				l.log,
				l.authCallback,
				l.textCallback,
				l.fileCallback,
				l.endCallback,
				l.rand,
			)
			go func() {
				defer func() { <-l.limit }()
				defer l.active.Delete(conn)
				c.Accept(l.wg)
			}()
		}
	}
}

func (l Listener) Stop() error {
	l.log.Debug("stopping listener...")
	if l.sock == nil {
		return nil
	}

	l.stopCh <- struct{}{}
	if err := l.sock.Close(); err != nil {
		return fmt.Errorf("close socket: %w", err)
	}
	l.active.Range(func(key, _ any) bool {
		_ = key.(net.Conn).Close()
		return true
	})
	l.wg.Wait()

	l.log.Debug("listener stopped")
	return nil
}

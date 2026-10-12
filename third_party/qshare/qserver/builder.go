package qserver

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"

	"github.com/ACLzz/qshare"
	"github.com/ACLzz/qshare/internal/adapter"
	"github.com/ACLzz/qshare/internal/ble"
	internalLog "github.com/ACLzz/qshare/internal/log"
	"github.com/ACLzz/qshare/internal/rand"
	"github.com/ACLzz/qshare/internal/tests/helper"
	"github.com/ACLzz/qshare/qserver/listener"

	"tinygo.org/x/bluetooth"
)

type serverBuilder struct {
	rand rand.Random

	hostname string
	port     int
	endpoint []byte
	adapter  *bluetooth.Adapter
	device   qshare.DeviceType
	logger   qshare.Logger

	isHostnameSet         bool
	isPortSet             bool
	isEndpointSet         bool
	isBluetoothAdapterSet bool
	isDeviceTypeSet       bool
	isLoggerSet           bool
	isRandomSet           bool
}

func NewBuilder() *serverBuilder {
	return &serverBuilder{}
}

// Add name of your server. It will appear in servers list on client.
func (b *serverBuilder) WithHostname(hostname string) *serverBuilder {
	b.hostname = hostname
	b.isHostnameSet = true
	return b
}

// Add port of your server.
func (b *serverBuilder) WithPort(port int) *serverBuilder {
	b.port = port
	b.isPortSet = true
	return b
}

// Add endpoint for mDNS. This must be exactly 4 bytes and
// match with a client endpoint id, if you use one.
func (b *serverBuilder) WithEndpoint(endpoint string) *serverBuilder {
	b.endpoint = []byte(endpoint)
	b.isEndpointSet = true
	return b
}

// Add bluetooth adapter of your choice.
// Don't forget to enable it beforehand.
func (b *serverBuilder) WithAdapter(adapter *bluetooth.Adapter) *serverBuilder {
	b.adapter = adapter
	b.isBluetoothAdapterSet = true
	return b
}

// Add specific device type. Default is Unknown.
func (b *serverBuilder) WithDeviceType(device qshare.DeviceType) *serverBuilder {
	b.device = device
	b.isDeviceTypeSet = true
	return b
}

// Add logger interface implementation to keep logs format consistent.
func (b *serverBuilder) WithLogger(logger qshare.Logger) *serverBuilder {
	b.logger = logger
	b.isLoggerSet = true
	return b
}

// Internal function. Shouldn't be used outside the library.
func (b *serverBuilder) WithRandom(rng rand.Random) *serverBuilder {
	b.rand = rng
	b.isRandomSet = true
	return b
}

// Build and return server if no errors occurred.
// All callbacks are required.
func (b *serverBuilder) Build(
	authCallback qshare.AuthCallback,
	textCallback qshare.TextCallback,
	fileCallback qshare.FileCallback,
	endCallback qshare.EndCallback,
) (*Server, error) {
	if err := b.propagateDefaultValues(); err != nil {
		return nil, fmt.Errorf("propagate default values: %w", err)
	}

	var (
		err   error
		bleAD *bluetooth.Advertisement
	)
	if !helper.IsCI() {
		bleAD, err = ble.NewAdvertisement(b.adapter, b.rand)
		if err != nil {
			return nil, fmt.Errorf("create ble advertisement: %w", err)
		}
	}

	lisnr, err := listener.New(
		b.port,
		b.logger,
		authCallback,
		textCallback,
		fileCallback,
		endCallback,
		b.rand,
	)
	if err != nil {
		return nil, fmt.Errorf("create listener: %w", err)
	}

	return &Server{
		bleAD:    bleAD,
		listener: lisnr,
		conf: serverConfig{
			name: newName(b.endpoint),
			port: b.port,
			txt: base64.RawURLEncoding.EncodeToString(
				adapter.CraftEndpointInfo(
					b.rand,
					b.hostname,
					b.device,
				),
			),
		},
	}, nil
}

type propagateValueFn func() error

func (b *serverBuilder) propagateDefaultValues() error {
	funcs := []propagateValueFn{
		b.propRandom,
		b.propLogger,
		b.propHostname,
		b.propPort,
		b.propAdapter,
		b.propEndpoint,
		b.propDeviceType,
	}

	var err error
	for i := range funcs {
		err = funcs[i]()
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *serverBuilder) propHostname() error {
	if b.isHostnameSet {
		return nil
	}

	var err error
	b.hostname, err = os.Hostname()
	if err != nil {
		return fmt.Errorf("get os hostname: %w", err)
	}

	return nil
}

func (b *serverBuilder) propPort() error {
	if b.isPortSet {
		if b.port <= 1024 {
			return ErrInvalidPort
		}

		return nil
	}

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("open tcp socket: %w", err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			b.logger.Error("close listener", err)
		}
	}()

	b.port = l.Addr().(*net.TCPAddr).Port
	return nil
}

func (b *serverBuilder) propAdapter() error {
	if b.isBluetoothAdapterSet {
		if b.adapter == nil {
			return ErrInvalidAdapter
		}
	} else {
		b.adapter = bluetooth.DefaultAdapter
	}
	if !helper.IsCI() {
		if err := b.adapter.Enable(); err != nil {
			return fmt.Errorf("%w: enable bluetooth adapter: %w", ErrInvalidAdapter, err)
		}
	}
	return nil
}

func (b *serverBuilder) propEndpoint() error {
	if b.isEndpointSet {
		if len(b.endpoint) != 4 {
			return ErrInvalidEndpoint
		}
		return nil
	}

	b.endpoint = rand.AlphaNum(b.rand, 4)
	return nil
}

func (b *serverBuilder) propDeviceType() error {
	if b.isDeviceTypeSet {
		if !b.device.IsValid() {
			return ErrInvalidDeviceType
		}
		return nil
	}

	b.device = qshare.UnknownDevice
	return nil
}

func (b *serverBuilder) propLogger() error {
	if b.isLoggerSet {
		if b.logger == nil {
			return ErrInvalidLogger
		}
		return nil
	}

	b.logger = internalLog.NewLogger()
	return nil
}

func (b *serverBuilder) propRandom() error {
	if b.isRandomSet {
		return nil
	}

	b.rand = rand.NewCrypt()
	return nil
}

func newName(endpoint []byte) string {
	name := make([]byte, 10)

	name[0] = 0x23                           // PCP
	copy(name[1:], endpoint)                 // endpoint
	copy(name[5:], []byte{0xFC, 0x9F, 0x5E}) // service ID

	return base64.RawURLEncoding.EncodeToString(name)
}

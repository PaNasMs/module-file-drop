package qserver

import (
	"fmt"
	"strings"

	"github.com/ACLzz/qshare/internal/mdns"
	"github.com/ACLzz/qshare/internal/tests/helper"
	"github.com/ACLzz/qshare/qserver/listener"
	"github.com/grandcat/zeroconf"
	"tinygo.org/x/bluetooth"
)

type (
	// Server instance manage all required services to make your
	// machine discoverable by other quick share clients.
	Server struct {
		conf       serverConfig
		bleAD      *bluetooth.Advertisement
		mDNSServer *zeroconf.Server
		listener   listener.Listener
	}

	serverConfig struct {
		name string
		txt  string
		port int
	}
)

// Start listener and other required services.
func (s *Server) Listen() error {
	if err := s.listen(); err != nil {
		if stopErr := s.Stop(); stopErr != nil {
			return fmt.Errorf("stopping server on failed start: %w: %w", stopErr, err)
		}

		return err
	}

	return nil
}

func (s *Server) listen() error {
	var err error
	s.mDNSServer, err = zeroconf.Register(
		s.conf.name,
		mdns.MDNsServiceType,
		"local.",
		s.conf.port,
		[]string{"n=" + s.conf.txt},
		nil,
	)
	if err != nil {
		return fmt.Errorf("start mDNS server: %w", err)
	}

	go s.listener.Listen()

	// skip ble advertisement start for ci
	if !helper.IsCI() {
		if err := s.bleAD.Start(); err != nil {
			return fmt.Errorf("start ble advertisement: %w", err)
		}
	}

	return nil
}

// Gracefully shut down all services.
func (s *Server) Stop() error {
	var err, gErr error
	if s.mDNSServer != nil {
		s.mDNSServer.Shutdown()
	}

	if err = s.listener.Stop(); err != nil {
		gErr = fmt.Errorf("%w: stop listener: %w", gErr, err)
	}

	// skip ble advertisement stop for ci
	if !helper.IsCI() {
		if err = s.bleAD.Stop(); err != nil &&
			!strings.Contains(err.Error(), "advertisement is not started") {
			gErr = fmt.Errorf("%w: stop ble advertisement: %w", gErr, err)
		}
	}

	return gErr
}

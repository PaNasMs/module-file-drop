package listener

import (
	"fmt"

	adapter "github.com/ACLzz/qshare/internal/adapter"
)

func (c *connection) processConnRequest(msg []byte) error {
	// just validate and let it go
	_, err := c.adapter.UnmarshalConnRequest(msg)
	if err != nil {
		return err
	}

	return nil
}

func (c *connection) processUKEYInitMessage(msg []byte) error {
	if err := c.adapter.ValidateClientInit(msg); err != nil {
		return err
	}
	if err := c.adapter.SendServerInit(); err != nil {
		return err
	}
	return nil
}

func (c *connection) processConnResponse(msg []byte) error {
	resp, err := c.adapter.UnmarshalConnResponse(msg)
	if err != nil {
		return err
	}

	// validate
	if !resp.IsConnAccepted {
		return adapter.ErrConnWasEndedByClient
	}

	// send response
	if err = c.adapter.SendConnResponse(true); err != nil {
		return fmt.Errorf("send connection response")
	}

	// init connection phase done. assign pairing phase and setup cipher
	if err = c.adapter.EnableEncryption(); err != nil {
		return fmt.Errorf("enable adapter encryption: %w", err)
	}
	c.phase = pairing_phase
	return nil
}

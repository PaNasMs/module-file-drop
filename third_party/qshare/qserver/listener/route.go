package listener

import (
	"errors"

	adapter "github.com/ACLzz/qshare/internal/adapter"
)

func (c *connection) route(msg []byte) {
	var err error
	nextMessage := c.nextExpectedMessage
	switch c.phase {
	case init_phase:
		switch c.nextExpectedMessage {
		// connection request
		case conn_request:
			err = c.processConnRequest(msg)
			nextMessage = client_init
		// client init
		case client_init:
			err = c.processUKEYInitMessage(msg)
			nextMessage = client_finish
		// client finish
		case client_finish:
			err = c.adapter.ValidateClientFinished(msg)
			nextMessage = conn_response
		// connection response
		case conn_response:
			err = c.processConnResponse(msg)
			nextMessage = paired_key_encryption
		}
	case pairing_phase:
		switch c.nextExpectedMessage {
		// paired key encryption
		case paired_key_encryption:
			err = c.processPairedKeyEncryption(msg)
			nextMessage = paired_key_result
		// paired key result
		case paired_key_result:
			err = c.processPairedKeyResult(msg)
			nextMessage = introduction
		}
	case transfer_phase:
		switch c.nextExpectedMessage {
		// transfer introduction
		case introduction:
			err = c.processIntroduction(msg)
			nextMessage = accept_reject
		// accept or reject response
		case accept_reject:
			err = c.processTransferRequest(msg)
			nextMessage = transfer_start
		// start transfer
		case transfer_start:
			c.adapter.EnableTransferHandler(c.textMeta)
		}
	}

	if err != nil {
		// check if message was actually service message
		innerErr := c.adapter.ProcessServiceMessage(msg)
		if innerErr == nil {
			return
		}

		// cancel context if conn was ended by client
		if errors.Is(innerErr, adapter.ErrConnWasEndedByClient) {
			c.cancelCtx()
			c.log.Debug("conn was ended by client")
			return
		}

		// notify client if message was invalid
		if errors.Is(err, adapter.ErrInvalidMessage) {
			if c.nextExpectedMessage <= conn_response {
				c.adapter.SendBadMessageError()
			}
			c.log.Warn("got invalid message", "expectedMessage", c.nextExpectedMessage)
			return
		}

		c.log.Error("process message", err)
		return
	}

	c.nextExpectedMessage = nextMessage
}

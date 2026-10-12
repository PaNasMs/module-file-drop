package listener

func (c *connection) processPairedKeyEncryption(msg []byte) error {
	if err := c.adapter.ValidatePairedKeyEncryption(msg); err != nil {
		return err
	}

	return c.adapter.SendPairedKeyEncryption()
}

func (c *connection) processPairedKeyResult(msg []byte) error {
	if err := c.adapter.ValidatePairedKeyResult(msg); err != nil {
		return err
	}

	if err := c.adapter.SendPairedKeyResult(); err != nil {
		return err
	}

	c.phase = transfer_phase
	return nil
}

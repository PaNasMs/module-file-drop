package listener

import "errors"

var (
	ErrInternalError           = errors.New("internal error")
	ErrTextTransferNotExpected = errors.New("text transfer was not expected")
	ErrTransferNotComplete     = errors.New("transfer was flagged as completed, " +
		"but didn't transfer all bytes")
)

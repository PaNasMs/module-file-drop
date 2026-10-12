package adapter

import "errors"

var (
	ErrConnWasEndedByClient = errors.New("connection was ended by client")
	ErrNotServiceMessage    = errors.New("message is not service message")
	ErrInvalidMessageLength = errors.New("invalid message length")
	ErrMessageTooLong       = errors.New("message to long")
	ErrOffsetMismatch       = errors.New("new chunk offset not expected")
	ErrFetchFullMessage     = errors.New("cannot fetch full message")
)

var (
	ErrInvalidSignatureScheme  = errors.New("got unsupported signature scheme")
	ErrInvalidIV               = errors.New("got invalid iv in header")
	ErrInvalidEncryptionScheme = errors.New("got unsupported encryption scheme")
)

var (
	ErrInvalidMessage      = errors.New("got invalid message")
	ErrInvalidOfflineFrame = errors.New("offline frame is invalid")
	ErrInvalidSharingFrame = errors.New("sharing frame is invalid")
)

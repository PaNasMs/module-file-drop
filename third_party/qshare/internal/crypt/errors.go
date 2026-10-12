package crypt

import "errors"

var ErrInvalidCipher = errors.New("cipher was not prepared for setup")

var (
	ErrInvalidSenderInitMessage = errors.New("invalid sender init message")
	ErrInvalidSenderPublicKey   = errors.New("invalid sender public key")
)

var (
	ErrInvalidReceiverInitMessage = errors.New("invalid receiver init message")
	ErrInvalidReceiverPrivateKey  = errors.New("invalid receiver private key")
)

var (
	ErrInvalidSecureMessageSignature = errors.New("got invalid signature for secure message")
)

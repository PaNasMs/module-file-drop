package qserver

import "errors"

// builder
var (
	ErrInvalidPort          = errors.New("invalid port value")
	ErrInvalidAdapter       = errors.New("invalid bluetooth adapter")
	ErrInvalidAdvertisement = errors.New("invalid bluetooth advertisement")
	ErrInvalidLogger        = errors.New("invalid logger")
	ErrInvalidEndpoint      = errors.New("invalid endpoint")
	ErrInvalidDeviceType    = errors.New("invalid device type")
)

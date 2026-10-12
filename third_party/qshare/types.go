package qshare

import "io"

type DeviceType uint

const (
	UnknownDevice DeviceType = iota
	PhoneDevice
	TabletDevice
	LaptopDevice
)

func (t DeviceType) IsValid() bool {
	return t <= LaptopDevice
}

type TextType uint8

const (
	TextUnknown TextType = iota
	TextText
	TextURL
	TextAddress
	TextPhoneNumber
)

func (t TextType) IsUnknown() bool {
	return t == TextUnknown || t > TextPhoneNumber
}

type (
	TextMeta struct {
		Type  TextType
		Title string
		Size  int64
	}
	TextPayload struct {
		Meta TextMeta
		Text string
	}
)

type FileType uint8

const (
	FileUnknown FileType = iota
	FileImage
	FileVideo
	FileApp
	FileAudio
)

type (
	FileMeta struct {
		Type     FileType
		Name     string
		MimeType string
		Size     int64
	}
	FilePayload struct {
		Meta FileMeta
		Pr   *io.PipeReader
		Pw   *io.PipeWriter
	}
)

type (
	TextCallback func(payload TextPayload)
	FileCallback func(payload FilePayload)
	AuthCallback func(text *TextMeta, files []FileMeta, pin uint16) bool
	EndCallback  func(complete bool)
)

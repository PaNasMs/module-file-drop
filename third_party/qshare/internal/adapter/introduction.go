package adapter

import (
	"math/rand/v2"

	"github.com/ACLzz/qshare"
	pbSharing "github.com/ACLzz/qshare/internal/protobuf/gen/sharing"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

type IntroductionFrame struct {
	Text  *TextMeta
	Files map[int64]*qshare.FilePayload
}

func (f IntroductionFrame) HasText() bool {
	return f.Text != nil
}

func (f IntroductionFrame) HasFiles() bool {
	return len(f.Files) > 0
}

/*
TODO: SECURITY:
- sanitize all strings
*/
func (a *Adapter) UnmarshalIntroduction(msg []byte) (IntroductionFrame, error) {
	frame, err := unmarshalSharingFrame(msg)
	if err != nil {
		return IntroductionFrame{}, err
	}

	if frame.GetType() != pbSharing.V1Frame_INTRODUCTION || frame.GetIntroduction() == nil {
		return IntroductionFrame{}, ErrInvalidMessage
	}

	// validate that at least one text or file are sent
	text := frame.GetIntroduction().GetTextMetadata()
	files := frame.GetIntroduction().GetFileMetadata()
	if len(files) == 0 && len(text) == 0 {
		return IntroductionFrame{}, ErrInvalidMessage
	}

	return IntroductionFrame{
		Text:  mapPBTextPayloadToQShare(text),
		Files: mapPBFilePayloadsToQShare(files),
	}, nil
}

func (a *Adapter) SendIntroduction(frame IntroductionFrame) error {
	return a.writeSecureFrame(&pbSharing.V1Frame{
		Type: pbSharing.V1Frame_INTRODUCTION.Enum(),
		Introduction: &pbSharing.IntroductionFrame{
			TextMetadata: mapQShareTextPayloadToPB(frame.Text),
			FileMetadata: mapQShareFilePayloadsToPB(frame.Files),
		},
	})
}

type TextMeta struct {
	qshare.TextMeta
	ID int64
}

func (meta *TextMeta) GetQShareTextMeta() *qshare.TextMeta {
	if meta == nil {
		return nil
	}
	return &meta.TextMeta
}

func NewTextMeta(ID int64, t qshare.TextType, title string, size int64) *TextMeta {
	return &TextMeta{
		TextMeta: qshare.TextMeta{
			Type:  t,
			Title: title,
			Size:  size,
		},
		ID: ID,
	}
}

func mapPBTextPayloadToQShare(payloads []*pbSharing.TextMetadata) *TextMeta {
	if len(payloads) == 0 {
		return nil
	}

	var textType qshare.TextType
	switch payloads[0].GetType() {
	case pbSharing.TextMetadata_TEXT:
		textType = qshare.TextText
	case pbSharing.TextMetadata_URL:
		textType = qshare.TextURL
	case pbSharing.TextMetadata_ADDRESS:
		textType = qshare.TextAddress
	case pbSharing.TextMetadata_PHONE_NUMBER:
		textType = qshare.TextPhoneNumber
	default:
		textType = qshare.TextUnknown
	}

	return &TextMeta{
		TextMeta: qshare.TextMeta{
			Type:  textType,
			Title: payloads[0].GetTextTitle(),
			Size:  payloads[0].GetSize(),
		},
		ID: *payloads[0].PayloadId,
	}
}

func mapQShareTextPayloadToPB(text *TextMeta) []*pbSharing.TextMetadata {
	if text == nil {
		return nil
	}

	var textType pbSharing.TextMetadata_Type
	switch text.Type {
	case qshare.TextText:
		textType = pbSharing.TextMetadata_TEXT
	case qshare.TextURL:
		textType = pbSharing.TextMetadata_URL
	case qshare.TextAddress:
		textType = pbSharing.TextMetadata_ADDRESS
	case qshare.TextPhoneNumber:
		textType = pbSharing.TextMetadata_PHONE_NUMBER
	default:
		textType = pbSharing.TextMetadata_UNKNOWN
	}

	return []*pbSharing.TextMetadata{
		{
			TextTitle: proto.String(text.Title),
			Type:      textType.Enum(),
			PayloadId: proto.Int64(text.ID),
			Size:      proto.Int64(text.Size),
			Id:        proto.Int64(rand.Int64()),
		},
	}
}

func mapPBFilePayloadsToQShare(payloads []*pbSharing.FileMetadata) map[int64]*qshare.FilePayload {
	var fileType qshare.FileType
	mappedPayloads := make(map[int64]*qshare.FilePayload, len(payloads))

	for i := range payloads {
		switch payloads[i].GetType() {
		case pbSharing.FileMetadata_IMAGE:
			fileType = qshare.FileImage
		case pbSharing.FileMetadata_VIDEO:
			fileType = qshare.FileVideo
		case pbSharing.FileMetadata_APP:
			fileType = qshare.FileApp
		case pbSharing.FileMetadata_AUDIO:
			fileType = qshare.FileAudio
		default:
			fileType = qshare.FileUnknown
		}

		mappedPayloads[payloads[i].GetPayloadId()] = &qshare.FilePayload{
			Meta: qshare.FileMeta{
				Type:     fileType,
				Name:     payloads[i].GetName(),
				MimeType: payloads[i].GetMimeType(),
				Size:     payloads[i].GetSize(),
			},
			Pr: nil,
		}
	}

	return mappedPayloads
}

func mapQShareFilePayloadsToPB(payloads map[int64]*qshare.FilePayload) []*pbSharing.FileMetadata {
	var fileType pbSharing.FileMetadata_Type
	mappedPayloads := make([]*pbSharing.FileMetadata, 0, len(payloads))

	for id := range payloads {
		switch payloads[id].Meta.Type {
		case qshare.FileImage:
			fileType = pbSharing.FileMetadata_IMAGE
		case qshare.FileVideo:
			fileType = pbSharing.FileMetadata_VIDEO
		case qshare.FileApp:
			fileType = pbSharing.FileMetadata_APP
		case qshare.FileAudio:
			fileType = pbSharing.FileMetadata_AUDIO
		default:
			fileType = pbSharing.FileMetadata_UNKNOWN
		}
		mappedPayloads = append(mappedPayloads, &pbSharing.FileMetadata{
			Name:      proto.String(payloads[id].Meta.Name),
			Type:      fileType.Enum(),
			PayloadId: proto.Int64(id),
			Size:      proto.Int64(payloads[id].Meta.Size),
			MimeType:  proto.String(payloads[id].Meta.MimeType),
			Id:        proto.Int64(int64(uuid.New().ID())),
		})
	}

	return mappedPayloads
}

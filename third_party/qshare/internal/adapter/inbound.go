package adapter

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode"

	pbConnections "github.com/ACLzz/qshare/internal/protobuf/gen/connections"
	pbSecureMessage "github.com/ACLzz/qshare/internal/protobuf/gen/securemessage"
	pbSharing "github.com/ACLzz/qshare/internal/protobuf/gen/sharing"
	"google.golang.org/protobuf/proto"
)

// TODO: fine tune all of the buffer sizes
const (
	MAX_MESSAGE_LENGTH = 5 * 1024 * 1024 // 5Mb
	MAX_TEXT_LENGTH    = 5 * 1024 * 1024 // 5Mb
)

var errTransferInProgress = errors.New("transfer in progress")

type FileChunk struct {
	FileID       int64
	IsFinalChunk bool
	Body         []byte
}

func unmarshalOfflineFrame(msg []byte) (*pbConnections.V1Frame, error) {
	var offFrame pbConnections.OfflineFrame
	if err := proto.Unmarshal(msg, &offFrame); err != nil {
		return nil, fmt.Errorf("unmarshal offline frame: %w", err)
	}

	if offFrame.GetVersion() != pbConnections.OfflineFrame_V1 || offFrame.GetV1() == nil {
		return nil, ErrInvalidOfflineFrame
	}

	return offFrame.GetV1(), nil
}

func unmarshalSharingFrame(msg []byte) (*pbSharing.V1Frame, error) {
	var sharingFrame pbSharing.Frame
	if err := proto.Unmarshal(msg, &sharingFrame); err != nil {
		return nil, fmt.Errorf("unmarshal sharing frame: %w", err)
	}

	if sharingFrame.GetVersion() != pbSharing.Frame_V1 || sharingFrame.GetV1() == nil {
		return nil, ErrInvalidSharingFrame
	}

	return sharingFrame.GetV1(), nil
}

func (a *Adapter) decryptMessage(msg []byte) ([]byte, error) {
	// unmarshal secure message
	var secMsg pbSecureMessage.SecureMessage
	if err := proto.Unmarshal(msg, &secMsg); err != nil {
		return nil, fmt.Errorf("unmarshal secure message: %w", err)
	}
	if err := a.cipher.ValidateSignature(secMsg.GetHeaderAndBody(), secMsg.GetSignature()); err != nil {
		return nil, err
	}

	// unmarshal header and body
	var hb pbSecureMessage.HeaderAndBody
	if err := proto.Unmarshal(secMsg.GetHeaderAndBody(), &hb); err != nil {
		return nil, fmt.Errorf("unmarshal secure message: %w", err)
	}
	if hb.GetHeader().GetEncryptionScheme() != pbSecureMessage.EncScheme_AES_256_CBC {
		return nil, ErrInvalidEncryptionScheme
	}
	if hb.GetHeader().GetSignatureScheme() != pbSecureMessage.SigScheme_HMAC_SHA256 {
		return nil, ErrInvalidSignatureScheme
	}
	if len(hb.GetHeader().GetIv()) < 16 {
		return nil, ErrInvalidIV
	}

	// decrypt header and body into device to device message
	d2dMsg, err := a.cipher.Decrypt(&hb)
	if err != nil {
		return nil, fmt.Errorf("decrypt message: %w", err)
	}

	return d2dMsg.GetMessage(), nil
}

type (
	FileChunkCallback    func(chunk FileChunk) error
	TextTransferCallback func(text string) error
)

/*
TODO: SECURITY:
- implement timeouts between messages
*/
func (a *Adapter) Reader(ctx context.Context) func() ([]byte, error) {
	var (
		err            error
		handleTransfer = a.transferHandler()
		msgLen         uint32
		lenBuf         = make([]byte, 4)
		clearBuf       = func() {
			lenBuf = make([]byte, 4)
		}
	)

	return func() ([]byte, error) {
		for {
			select {
			case <-ctx.Done():
				return nil, io.EOF
			default:
				// fetch length of the upcoming message
				if _, err = io.ReadFull(a.conn, lenBuf); err != nil {
					if errors.Is(err, io.EOF) {
						a.log.Debug("conn ended abruptly")
						return nil, ErrConnWasEndedByClient
					}

					a.log.Error("read msg length", err)
					clearBuf()
					return nil, ErrInvalidMessageLength
				}
				msgLen = binary.BigEndian.Uint32(lenBuf)
				clearBuf()

				if msgLen > MAX_MESSAGE_LENGTH {
					a.log.Error("message is too long", nil, "length", msgLen)
					return nil, ErrMessageTooLong
				}

				// fetch message in bytes
				msgBuf := make([]byte, msgLen)
				if _, err = io.ReadFull(a.conn, msgBuf); err != nil {
					a.log.Error("fetch message", err)
					return nil, ErrFetchFullMessage
				}

				// decrypt message if isEncrypted is set
				if a.isEncrypted {
					msgBuf, err = a.decryptMessage(msgBuf)
					if err != nil {
						return nil, fmt.Errorf("decrypt message: %w", err)
					}
				}

				msgBuf, err = handleTransfer(msgBuf)
				if err != nil {
					if errors.Is(err, errTransferInProgress) {
						continue
					}
					return nil, fmt.Errorf("handle transfer: %w", err)
				}

				return msgBuf, nil
			}
		}
	}
}

/*
TODO: SECURITY:
- sanitize all strings from client
- implement timeouts between chunks
- implememnt timeouts between files and texts
*/
func (a *Adapter) transferHandler() func(msg []byte) ([]byte, error) {
	var (
		frame       *pbConnections.V1Frame
		err         error
		header      *pbConnections.PayloadTransferFrame_PayloadHeader
		chunk       *pbConnections.PayloadTransferFrame_PayloadChunk
		fileOffsets = map[int64]int64{}
		buf         = make([]byte, 0, MAX_TEXT_LENGTH)
		clearBuf    = func() {
			buf = make([]byte, 0, MAX_TEXT_LENGTH)
		}
	)

	return func(msg []byte) ([]byte, error) {
		frame, err = unmarshalOfflineFrame(msg)
		if err != nil {
			return msg, nil // ignore non-offline frames
		}

		if frame.GetType() != pbConnections.V1Frame_PAYLOAD_TRANSFER ||
			frame.GetPayloadTransfer() == nil {
			return msg, nil // ignore non-payload transfer frames
		}

		header = frame.GetPayloadTransfer().GetPayloadHeader()
		chunk = frame.GetPayloadTransfer().GetPayloadChunk()
		isFinalChunk := chunk.GetFlags()&1 == 1

		if header.GetType() == pbConnections.PayloadTransferFrame_PayloadHeader_FILE {
			// for files pass the data to the callback
			if fileOffsets[header.GetId()] != chunk.GetOffset() {
				return nil, ErrInvalidMessage
			}

			if err = a.fileChunkCallback(FileChunk{
				FileID:       header.GetId(),
				IsFinalChunk: isFinalChunk,
				Body:         chunk.GetBody(),
			}); err != nil {
				return nil, fmt.Errorf("file chunk callback: %w", err)
			}
			fileOffsets[header.GetId()] += int64(len(chunk.GetBody()))
			return nil, errTransferInProgress
		} else if header.GetType() == pbConnections.PayloadTransferFrame_PayloadHeader_BYTES {
			// for texts read full text and pass to the reader
			if chunk.GetOffset() != int64(len(buf)) {
				return nil, ErrInvalidMessage
			}
			if len(chunk.GetBody()) > MAX_TEXT_LENGTH-len(buf) {
				return nil, ErrMessageTooLong
			}
			if len(chunk.GetBody()) > 0 {
				buf = append(buf, chunk.GetBody()...)
			}

			if isFinalChunk {
				bufCopy := make([]byte, len(buf))
				copy(bufCopy, buf)
				clearBuf()
				if a.isTransfer && header.GetId() == a.textPayloadID {
					return nil, a.textTransferCallback(cleanTextTransfer(bufCopy))
				}
				return bufCopy, nil
			}
			return nil, errTransferInProgress
		}

		return msg, nil
	}
}

func cleanTextTransfer(b []byte) string {
	var result []rune
	for _, r := range string(b) {
		if unicode.IsPrint(r) || r == '\t' || r == '\n' {
			result = append(result, r)
		}
	}
	return string(result)
}

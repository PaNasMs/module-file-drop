package listener

import (
	"fmt"
	"io"

	"github.com/ACLzz/qshare"
	adapter "github.com/ACLzz/qshare/internal/adapter"
)

func (c *connection) processIntroduction(msg []byte) error {
	intro, err := c.adapter.UnmarshalIntroduction(msg)
	if err != nil {
		return err
	}
	if len(intro.Files) > 128 {
		return adapter.ErrInvalidMessage
	}

	if intro.HasText() {
		c.textMeta = intro.Text
		c.expectedPayloads++
	}
	if intro.HasFiles() {
		files := map[int64]*filePayload{}
		for payloadID := range intro.Files {
			pr, pw := io.Pipe()
			files[payloadID] = &filePayload{
				Pd: qshare.FilePayload{
					Meta: intro.Files[payloadID].Meta,
					Pr:   pr,
					Pw:   pw,
				},
			}
		}
		c.filePayloads = files
		c.expectedPayloads += len(c.filePayloads)
	}

	return nil
}

func (c *connection) processTransferRequest(msg []byte) error {
	if err := c.adapter.ValidateTransferRequest(msg); err != nil {
		return err
	}

	files := []qshare.FileMeta{}
	for id := range c.filePayloads {
		files = append(files, c.filePayloads[id].Pd.Meta)
	}

	c.approved = c.authCallback(
		c.textMeta.GetQShareTextMeta(),
		files,
		c.adapter.Pin(),
	)
	return c.adapter.SendTransferResponse(c.approved)
}

func (c *connection) writeFileChunk(chunk adapter.FileChunk) error {
	file := c.filePayloads[chunk.FileID]
	if file == nil || file.Complete {
		return adapter.ErrInvalidMessage
	}
	if !file.IsNotified {
		c.filePayloads[chunk.FileID].IsNotified = true
		go c.fileCallback(file.Pd)
	}

	if len(chunk.Body) > 0 {
		n, err := file.Pd.Pw.Write(chunk.Body)
		if err != nil {
			return fmt.Errorf("write chunk to pipe: %w", err)
		}

		if n != len(chunk.Body) {
			return ErrInternalError // TODO: another error
		}
	}

	if chunk.IsFinalChunk {
		file.Complete = true
		if err := file.Pd.Pw.Close(); err != nil {
			c.log.Error("close file pipe writer", err)
		}
		c.log.Debug("file transfered", "filename", file.Pd.Meta.Name)

		c.receivedPayloads++
		c.checkIfLastPayload()
	}

	return nil
}

func (c *connection) writeText(text string) error {
	if c.textMeta == nil || c.textCallback == nil {
		return ErrTextTransferNotExpected
	}

	c.textCallback(qshare.TextPayload{
		Meta: c.textMeta.TextMeta,
		Text: text,
	})
	c.receivedPayloads++
	c.checkIfLastPayload()
	return nil
}

func (c *connection) checkIfLastPayload() {
	if c.receivedPayloads >= c.expectedPayloads {
		c.log.Debug("got last payload, disconnecting...")
		c.cancelCtx()
	}
}

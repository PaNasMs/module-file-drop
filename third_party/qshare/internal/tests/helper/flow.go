package helper

import (
	"github.com/ACLzz/qshare"
)

func StubAuthCallback(resp bool) qshare.AuthCallback {
	return func(text *qshare.TextMeta, files []qshare.FileMeta, pin uint16) bool {
		return resp
	}
}

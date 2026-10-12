package helper

import "github.com/ACLzz/qshare/internal/rand"

func RandomPort() int {
	return rand.NewCrypt().IntN(int(^uint16(0))-1024) + 1024
}

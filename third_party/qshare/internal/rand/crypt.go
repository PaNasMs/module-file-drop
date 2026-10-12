package rand

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
)

type Crypt struct{}

func NewCrypt() Crypt {
	return Crypt{}
}

func (r Crypt) IntN(max int) int {
	bigint, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err)
	}

	return int(bigint.Int64())
}

func (r Crypt) Int64() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}

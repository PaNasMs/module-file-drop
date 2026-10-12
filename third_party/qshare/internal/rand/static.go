package rand

import (
	"math/rand/v2"
)

type Static struct {
	rand rand.Rand
}

type source struct {
	seed uint64
}

func (s source) Uint64() uint64 {
	return s.seed
}

func NewStatic(seed int) Static {
	return Static{
		rand: *rand.New(source{seed: uint64(seed)}),
	}
}

func (r Static) IntN(max int) int {
	return r.rand.IntN(max)
}

func (r Static) Int64() int64 {
	return r.rand.Int64()
}

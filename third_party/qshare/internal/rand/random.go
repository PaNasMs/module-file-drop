package rand

type Random interface {
	IntN(max int) int
	Int64() int64
}

var (
	alphaNumRunes    = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	alphaNumSliceLen = len(alphaNumRunes)
)

func Bytes(r Random, size uint8) []byte {
	buf := make([]byte, size)
	for i := range size {
		buf[i] = byte(r.IntN(256))
	}

	return buf
}

func AlphaNum(r Random, size uint8) []byte {
	buf := make([]byte, size)
	for i := range size {
		buf[i] = byte(alphaNumRunes[r.IntN(alphaNumSliceLen)])
	}

	return buf
}

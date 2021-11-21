package load

import (
	"io"
	"math/rand"
)

type RequestGenerator interface {
	Generate() []byte
}
type RequestGeneratorFn func() []byte

func (f RequestGeneratorFn) Generate() []byte {
	return f()
}

var NoopRequestGenerator = RequestGeneratorFn(func() []byte {
	return nil
})

func NewRequestGenerator(body string, size int, variance int, seed int64) RequestGenerator {
	if body == "" && size == 0 {
		return NoopRequestGenerator
	}
	if body != "" {
		return RequestGeneratorFn(func() []byte {
			return []byte(body)
		})
	}
	r := rand.New(rand.NewSource(seed))
	return RequestGeneratorFn(func() []byte {
		s := size
		if variance > 0 {
			change := int(float64(rand.Intn(size)) * float64(variance) / 100.0)
			// larger or smaller?
			dir := rand.Intn(2) - 1
			s = size + dir * change
		}
		b := make([]byte, s)
		io.ReadFull(r, b)
		return b
	})

}

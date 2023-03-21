package load

import (
	"github.com/hashicorp/go-hclog"
	"testing"
	"time"
)

func Test0LoadGenerator(t *testing.T) {
	g := NewGenerator(2, 0.0, 0, 0, hclog.L())
	for i := 0; i < 10000; i++ {
		f := g.Generate()
		f()
	}
}

func Test50percent2CoresWithMemAllocLoadGenerator(t *testing.T) {
	g := NewGenerator(2, 50.0, 1024*1024, 2, hclog.L())
	for i := 0; i < 10; i++ {
		f := g.Generate()
		time.Sleep(1 * time.Millisecond)
		f()
	}
}

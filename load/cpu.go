package load

import (
	"runtime"
	"time"
)

// original code from:
// https://github.com/vikyd/go-cpu-load
// Thank you for your awesome work

// Finished should be called when a function exits to stop the load generation
type Finished func()

type Generator struct {
	coresCount float64
	percentage float64
}

func NewGenerator(cores, percentage float64) *Generator {
	return &Generator{cores, percentage}
}

// RunCPULoad run CPU load in specify cores count and percentage
func (g *Generator) Generate() Finished {
	if g.coresCount == 0 {
		return func() {}
	}

	finished := make(chan struct{})
	running := true

	go func() {
		runtime.GOMAXPROCS(int(g.coresCount))

		// second     ,s  * 1
		// millisecond,ms * 1000
		// microsecond,μs * 1000 * 1000
		// nanosecond ,ns * 1000 * 1000 * 1000

		// every loop : run + sleep = 1 unit

		// 1 unit = 100 ms may be the best
		var unitHundresOfMicrosecond float64 = 1000
		runMicrosecond := unitHundresOfMicrosecond * g.percentage
		sleepMicrosecond := unitHundresOfMicrosecond*100 - runMicrosecond
		for i := 0; i < int(g.coresCount); i++ {
			go func() {
				runtime.LockOSThread()
				// endless loop
				for running {
					begin := time.Now()
					for {
						// run 100%
						if time.Now().Sub(begin) > time.Duration(runMicrosecond)*time.Microsecond {
							break
						}
					}
					// sleep
					time.Sleep(time.Duration(sleepMicrosecond) * time.Microsecond)
				}
			}()
		}

		<-finished
	}()

	return func() {
		finished <- struct{}{}
		running = false
	}
}

package load

import (
	"math/rand"
	"runtime"
	"time"

	"github.com/hashicorp/go-hclog"
)

// original code from:
// https://github.com/vikyd/go-cpu-load
// Thank you for your awesome work

// Finished should be called when a function exits to stop the load generation
type Finished func()

type Generator struct {
	logger         hclog.Logger
	cpuCoresCount  float64
	cpuPercentage  float64
	memoryBytes    int
	memoryVariance int
	running        bool
	finished       chan struct{}
}

// NewGenerator creates a new load generator that can create atrificial memory and cpu pressure
func NewGenerator(cores, percentage float64, memoryBytes, memoryVariance int, logger hclog.Logger) *Generator {
	return &Generator{logger, cores, percentage, memoryBytes, memoryVariance, false, nil}
}

// Generate load for the request
func (g *Generator) Generate() Finished {
	// this needs to be a buffered channel or the return function will block and leak
	g.finished = make(chan struct{}, 2)
	g.running = true

	// generate the memory first to ensure that the CPU consumption
	// does not block memory creation
	g.generateMemory()
	g.generateCPU()

	return func() {
		// call finished twice for memory and CPU
		g.finished <- struct{}{}
		g.finished <- struct{}{}
		g.running = false
	}
}

// RunCPULoad run CPU load in specify cores count and percentage
func (g *Generator) generateCPU() {
	if g.cpuCoresCount == 0 {
		return
	}

	go func() {
		g.logger.Info("Generating CPU Load", "cores", g.cpuCoresCount, "percentage", g.cpuPercentage)

		runtime.GOMAXPROCS(int(g.cpuCoresCount))

		// second     ,s  * 1
		// millisecond,ms * 1000
		// microsecond,μs * 1000 * 1000
		// nanosecond ,ns * 1000 * 1000 * 1000

		// every loop : run + sleep = 1 unit

		// 1 unit = 100 ms may be the best
		var unitHundresOfMicrosecond float64 = 1000
		runMicrosecond := unitHundresOfMicrosecond * g.cpuPercentage
		sleepMicrosecond := unitHundresOfMicrosecond*100 - runMicrosecond
		for i := 0; i < int(g.cpuCoresCount); i++ {
			go func() {
				runtime.LockOSThread()
				// endless loop
				for g.running {
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

		// block until signal to complete load generation is received
		<-g.finished
	}()
}

func (g *Generator) generateMemory() {
	if g.memoryBytes == 0 {
		return
	}

	go func() {
		memLen := g.memoryBytes
		if g.memoryVariance > 0 {
			variance := rand.Intn(g.memoryVariance)
			memLen = g.memoryBytes * (variance / 100)
		}

		// allocate a slice of memory
		mem := make([]byte, 0, memLen)
		_ = mem

		// print the memory consumption
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		g.logger.Info("Allocated memory", "MB", bToMb(m.Alloc))

		// block until signal to complete load generation is received
		// mem should be deallocated when this function completes and will be
		// garbage collected
		<-g.finished

		//
		mem = nil

		// force go to collect the memory
		// it might be better to use malloc and dealloc here rather than th GC
		runtime.GC()
	}()
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

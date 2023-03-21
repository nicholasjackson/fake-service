package load

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
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
	cpuCoresCount  int
	cpuPercentage  float64
	memoryBytes    int
	memoryVariance int
}

// NewGenerator creates a new load generator that can create atrificial memory and cpu pressure
func NewGenerator(cores int, percentage float64, memoryBytes, memoryVariance int, logger hclog.Logger) *Generator {
	if percentage < 0 || percentage > 100 {
		panic(fmt.Errorf("got percentage: %f which is not between 0 and 100", percentage))
	}
	return &Generator{logger, cores, percentage, memoryBytes, memoryVariance}
}

// Generate load for the request
func (g *Generator) Generate() Finished {
	// generate the memory first to ensure that the CPU consumption
	// does not block memory creation
	finished := make(chan struct{})
	wg := sync.WaitGroup{}
	g.generateMemory(finished, &wg)
	g.generateCPU(finished, &wg)

	return func() {
		// call finished twice for memory and CPU
		close(finished)
		wg.Wait()
	}
}

// RunCPULoad run CPU load in specify cores count and percentage
func (g *Generator) generateCPU(finished chan struct{}, wg *sync.WaitGroup) {
	if g.cpuCoresCount == 0 || g.cpuPercentage == 0 {
		return
	}

	g.logger.Info("Generating CPU Load", "cores", g.cpuCoresCount, "percentage", g.cpuPercentage)

	runtime.GOMAXPROCS(g.cpuCoresCount)

	// second     ,s  * 1
	// millisecond,ms * 1000
	// microsecond,μs * 1000 * 1000
	// nanosecond ,ns * 1000 * 1000 * 1000

	// every loop : run + sleep = 1 unit

	// 1 unit = 100 ms may be the best
	var unitHundredOfMicrosecond = 1000
	runMicrosecond := int(math.Round(float64(unitHundredOfMicrosecond) * g.cpuPercentage))
	sleepMicrosecond := unitHundredOfMicrosecond*100 - runMicrosecond
	for i := 0; i < g.cpuCoresCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runtime.LockOSThread()
			// endless loop
			for {
				begin := time.Now()
				for {
					// run 100%
					if time.Now().Sub(begin) > time.Duration(runMicrosecond)*time.Microsecond {
						break
					}
				}
				select {
				case <-finished: // signal to complete load generation is received
					return
				case <-time.Tick(time.Duration(sleepMicrosecond) * time.Microsecond): // sleep
				}
			}
		}()
	}
}

func (g *Generator) generateMemory(finished chan struct{}, wg *sync.WaitGroup) {
	if g.memoryBytes == 0 {
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		memLen := g.memoryBytes
		if g.memoryVariance > 0 {
			// Varianj
			// 50 / 100 = 0.5
			// 0.5 *

			// variance is the max variance to apply
			// we need to apply a random variance that has a max of this number
			// e.g. Variance = 50 random is between 0-50
			variance := float64(rand.Intn(g.memoryVariance)) / 100

			// the memory variance should sometimes be larger and
			// sometimes be smaller
			direction := rand.Intn(2)

			variance = variance + float64(direction)

			memLen = int(float64(g.memoryBytes) * variance)
			g.logger.Info("Generate memory variance", "variance", variance, "direction", direction)
		}

		// allocate memory
		mem := make([]byte, 0, memLen)
		_ = mem

		// print the memory consumption
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		g.logger.Info("Allocated memory", "MB", bToMb(m.Alloc), "mem", memLen)

		// block until signal to complete load generation is received
		// mem should be deallocated when this function completes and will be
		// garbage collected
		<-finished

		// clean references
		mem = nil

		// force go to collect the memory
		// it might be better to use malloc and dealloc here rather than th GC
		//runtime.GC()
	}()
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

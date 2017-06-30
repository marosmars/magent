package aggregator

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"reflect"
	"sync"
	"testing"
	"time"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

const UUID = VppUuid("vpp")
const TIMEOUT = 300

var validConfigs = []BufferedAggregatorConfiguration{
	{
		InboundBufferSize:  1,
		OutboundBufferSize: 1,
		Name:               "Test",
	},
	{
		InboundBufferSize:  14564,
		OutboundBufferSize: 1468,
		Name:               "",
	},
	{
		InboundBufferSize:  -520,
		OutboundBufferSize: -4,
		Name:               "asdf908324l[;][][23;42;[];[23()*@&*^!&%$*!@(#&()",
	},
}

func TestCreateValidBuffered(t *testing.T) {
	for _, cfg := range validConfigs {

		if cfg.Create() == nil {
			t.Error("Unable to create buffered aggregator")
		}
	}
}

type testStruct struct {
	Name   string
	Number int
}

var stat = testStruct{"stat", 44}

func TestFlowBuffered(t *testing.T) {
	aggr := BufferedAggregatorConfiguration{
		InboundBufferSize:  0,
		OutboundBufferSize: 0,
		Name:               "Test",
	}.Create()

	aggr.Start(UUID)
	ch := make(chan (AggregatedStat))

	// Producer
	// Register outside of goroutine to make sure registration of producer is in place before collector pushes stat
	reg := aggr.Register()
	go func() {
		temp := <-reg.Channel()
		ch <- temp
		defer reg.Close()
	}()

	// Collector
	go func() {
		aggr.Channel() <- stat
	}()

	select {
	case a := <-ch:
		aggr.Close()

		if len(a.Stats()) != 1 &&
			!reflect.DeepEqual(a.Stats()[0], stat) {

			fmt.Println(a)
			t.Errorf("Received invalid value, expected: %v, received: %v", stat, a)
		}

	case <-time.After(time.Second * time.Duration(30)):
		t.Error("Timed out. Producer did not receive stat")
	}
}

func BenchmarkFlow0_0(b *testing.B) {
	benchmarkFlow(0, 0, b)
}
func BenchmarkFlow1_1(b *testing.B) {
	benchmarkFlow(1, 1, b)
}
func BenchmarkFlow100_1(b *testing.B) {
	benchmarkFlow(100, 1, b)
}
func BenchmarkFlow1_100(b *testing.B) {
	benchmarkFlow(1, 100, b)
}
func BenchmarkFlow100_100(b *testing.B) {
	benchmarkFlow(100, 100, b)
}

func benchmarkFlow(inSize int, outSize int, b *testing.B) {
	iterations := b.N
	b.Logf("Invoking benchmark flow for %v iterations, with in size: %v, out size: %v",
		iterations, inSize, outSize)

	// Aggregator
	aggr := BufferedAggregatorConfiguration{
		InboundBufferSize:  float64(inSize),
		OutboundBufferSize: float64(outSize),
		Name:               "Test",
	}.Create()
	aggr.Start(UUID)
	ch := make(chan (int))

	// Producer
	reg := aggr.Register()
	var count = 0
	go func() {
		for {
			temp := <-reg.Channel()
			count = count + len(temp.Stats())
			if count == iterations {
				ch <- count
				break
			}
		}
	}()

	// Consumer
	var iterationsPerRoutine []int
	var lock sync.Mutex
	b.RunParallel(func(pb *testing.PB) {
		var count = 0
		for pb.Next() {
			count = count + 1
			aggr.Channel() <- testStruct{"Statistic", count}
		}

		lock.Lock()
		defer lock.Unlock()

		iterationsPerRoutine = append(iterationsPerRoutine, count)
	})

	// Wait until everything gets through
	select {
	case <-ch:
		aggr.Close()
		b.Logf("Benchmark finished. Subroutine iterations: %v", iterationsPerRoutine)
	case <-time.After(time.Second * time.Duration(TIMEOUT)):
		b.Error("Timed out. Did not receive all stats")
	}
}

package aggregator

import (
	log "github.com/Sirupsen/logrus"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	log.SetLevel(log.ErrorLevel)
}

func TestFlowFiltering(t *testing.T) {
	aggr := FilteringAggregatorConfiguration{
		BufferedAggregatorConfiguration{
			InboundBufferSize:  0,
			OutboundBufferSize: 0,
			Name:               "Test",
		}}.Create()

	aggr.Start(UUID)
	ch := make(chan (AggregatedStat))

	// Producer
	reg := aggr.Register()
	go func() {
		defer reg.Close()

		temp := <-reg.Channel()
		ch <- temp
	}()

	// Consumer
	go func() {
		aggr.Channel() <- stat
		aggr.Channel() <- stat
		aggr.Channel() <- stat
		aggr.Channel() <- stat
	}()

	var expected int32 = 1
	var count int32 = 0

loop:
	for {
		select {
		case <-ch:
			atomic.AddInt32(&count, 1)
		case <-time.After(time.Second * time.Duration(5)):
			aggr.Close()
			if atomic.AddInt32(&count, 0) != expected {
				t.Errorf("Received unexpected number of stats, expected %v, received: %v", expected, count)
			}
			break loop
		}
	}
}

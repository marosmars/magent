package aggregator

import (
	"pnda/vpp/monitoring/util"
	"reflect"
)

// Configuration(builder) for a filtering aggregator.
// Based on buffered aggregator with the difference of filtering repeated identical stats.
type FilteringAggregatorConfiguration struct {
	BufferedAggregatorConfiguration
}

type filteringAggregator struct {
	*bufferedAggregator
	cache map[reflect.Type]interface{}
}

func (s FilteringAggregatorConfiguration) Create() Aggregator {
	cache := make(map[reflect.Type]interface{})
	delegateAggregator := &bufferedAggregator{
		inboundChannel: make(chan Stat, int(s.InboundBufferSize)),
		configuration:  s.BufferedAggregatorConfiguration,

		// Check if previous value received for the type is equal and if so ignore it
		stripCheck: func(stat Stat) bool {
			if value, isPresent := cache[reflect.TypeOf(stat)]; isPresent {

				// If equal with previous then discard
				if util.StringOf(value) == util.StringOf(stat) {
					return true
				}
			}

			// Cache and accept
			cache[reflect.TypeOf(stat)] = stat
			return false
		},
	}

	return &filteringAggregator{
		cache:              cache,
		bufferedAggregator: delegateAggregator,
	}
}

/*
Package aggregator provides aggregators(buffers) between consumers and producers.
*/
package aggregator

import (
	"time"
)

// Collector facing side of an aggregator
type CollectorAggregator interface {
	Channel() chan (Stat)
}

// Producer facing side of an aggregator
type ProducerAggregator interface {
	Register() ProducerRegistration
}

// A registration struct for producer to keep on to
type ProducerRegistration interface {
	Channel() chan (AggregatedStat)
	Close()
}

// Manager(upper layer) facing side of an aggregator
type ManagedAggregator interface {
	Start(uuid VppUuid)
	Close()
}

// All purpose aggregator
type Aggregator interface {
	CollectorAggregator
	ProducerAggregator
	ManagedAggregator
}

// All purpose aggregator configuration(builder)
type AggregatorConfiguration interface {
	Create() Aggregator
}

// Interface that each structure coming from a Consumer has to implement
type Stat interface {
}

// Wrapper stat including useful information in addition to the stat itself
type TimestampedStat struct {
	VppUuid   VppUuid   `json:"vpp_uuid"`
	Timestamp time.Time `json:"timestamp"`
	StatType  string    `json:"stat_type"`
	Stat      Stat      `json:"stat"`
}

// Collection of stats
type AggregatedStat interface {
	Stats() []TimestampedStat
}

type aggregatedStats struct {
	stats []TimestampedStat
}

// Returns the collected stats
func (s aggregatedStats) Stats() []TimestampedStat {
	return s.stats
}

// Vpp UUID type - uniquely identifying a VPP instance (host)
type VppUuid string

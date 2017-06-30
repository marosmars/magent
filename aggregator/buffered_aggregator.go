package aggregator

import (
	log "github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/util"
	"reflect"
	"sync"
	"time"
)

// Configuration(builder) for a buffered aggregator.
// Uses channels to transfer stats between collectors and producers.
type BufferedAggregatorConfiguration struct {
	Name               string
	InboundBufferSize  float64
	OutboundBufferSize float64
}

type bufferedAggregator struct {
	inboundChannel       chan (Stat)
	outboundChannelsLock sync.Mutex
	outboundChannels     [](chan (AggregatedStat))
	configuration        BufferedAggregatorConfiguration
	stripCheck           func(stat Stat) bool
}

func (s BufferedAggregatorConfiguration) Create() Aggregator {
	if s.InboundBufferSize < 0 {
		log.WithField("configuration", util.StringOf(s)).Warn("Invalid inbound channel size, setting to 0")
		s.InboundBufferSize = 0
	}

	if s.OutboundBufferSize < 0 {
		log.WithField("configuration", util.StringOf(s)).Warn("Invalid outbound channel size, setting to 0")
		s.OutboundBufferSize = 0
	}

	return &bufferedAggregator{
		inboundChannel: make(chan Stat, int(s.InboundBufferSize)),
		configuration:  s,
		stripCheck: func(stat Stat) bool {
			return false
		},
	}
}

func (s *bufferedAggregator) Channel() chan (Stat) {
	return s.inboundChannel
}

func (s *bufferedAggregator) Register() ProducerRegistration {
	s.outboundChannelsLock.Lock()
	defer s.outboundChannelsLock.Unlock()

	newChannel := make(chan AggregatedStat, int(s.configuration.OutboundBufferSize))
	s.outboundChannels = append(s.outboundChannels, newChannel)
	index := len(s.outboundChannels) - 1

	return &registration{
		channel: newChannel,
		closeLambda: func() {
			s.outboundChannelsLock.Lock()
			defer s.outboundChannelsLock.Unlock()

			// Remove and close channel from producer
			s.outboundChannels = append(s.outboundChannels[:index], s.outboundChannels[index+1:]...)
			close(newChannel)
		},
	}
}

type registration struct {
	channel     chan (AggregatedStat)
	closeLambda func()
}

func (s *registration) Channel() chan (AggregatedStat) {
	return s.channel
}

func (s *registration) Close() {
	s.closeLambda()
}

func (s *bufferedAggregator) Start(uuid VppUuid) {
	go s.collect(uuid)
}

func (s *bufferedAggregator) Close() {
	close(s.inboundChannel)

	s.outboundChannelsLock.Lock()
	defer s.outboundChannelsLock.Unlock()

	for i := 0; i < len(s.outboundChannels); i++ {
		close(s.outboundChannels[i])
	}

	s.outboundChannels = nil
}

func (s *bufferedAggregator) collect(uuid VppUuid) {

infiniteLoop:
	for {
		var statSlice []TimestampedStat
		var more bool

	collectLoop:
		for i := 0; i <= len(s.Channel()); i++ {
			var stat Stat
			stat, more = <-s.Channel()

			// In case of channel close, proceed to forward
			if !more {
				break collectLoop
			}

			if s.stripCheck(stat) {
				log.WithFields(log.Fields{
					"type": reflect.TypeOf(stat),
					"stat": util.StringOf(stat),
				}).Debug("Ignoring stat")
				continue
			}

			log.WithField("stat", util.StringOf(stat)).Info("Stat received, aggregating")
			statSlice = append(statSlice, TimestampedStat{
				VppUuid:   uuid,
				Timestamp: time.Now(),
				StatType:  reflect.TypeOf(stat).String(),
				Stat:      stat,
			})
		}

		// Ignore empty slices
		if len(statSlice) == 0 {
			continue infiniteLoop
		}

		log.WithField("batch-size", len(statSlice)).Info("Batch aggregated. Forwarding")
		s.forward(aggregatedStats{stats: statSlice})

		// In case of channel close, shut this collecting goroutine down
		if !more {
			log.Info("Inbound channel closed")
			break infiniteLoop
		}
	}
}

func (s *bufferedAggregator) forward(aggrStats AggregatedStat) {
	s.outboundChannelsLock.Lock()
	defer s.outboundChannelsLock.Unlock()

	if s.outboundChannels == nil {
		// Closed already
		return
	}

	for i := 0; i < len(s.outboundChannels); i++ {
		s.outboundChannels[i] <- aggrStats
	}
}

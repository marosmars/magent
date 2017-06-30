package producer

import (
	"github.com/Sirupsen/logrus"
	"io"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/util"
)

type FileProducerConfiguration struct {
	formatConfiguration
	rollingFileConfiguration
	Name string
}

func (s FileProducerConfiguration) Create() Producer {
	s.validateFormat(s)

	return &fileProducer{
		configuration: s,
		writer:        s.validateAndGetWriter(s),
	}
}

type fileProducer struct {
	registeredProducer
	configuration FileProducerConfiguration
	writer        io.WriteCloser
}

func (s *fileProducer) Start(aggregator aggregator.ProducerAggregator) {
	s.register(aggregator)
	s.writer.Close()
	go s.produce(aggregator)
}

func (s *fileProducer) Close() {
	s.registeredProducer.close()
	s.writer.Close()
}

func (s *fileProducer) produce(aggregator aggregator.ProducerAggregator) {
	failedCounter := 0

	for {
		aggrStats := <-s.reg.Channel()
		stats := aggrStats.Stats()

		for i := 0; i < len(stats); i++ {
			if _, err := s.writer.Write(s.encode(stats[i])); err != nil {
				logrus.WithFields(logrus.Fields{
					"component": util.StringOf(s),
					"error":     err,
				}).Warn("Unable to write stat. Ignoring")

				// Increase failedCounter and panic if too high
				failedCounter++
				if failedCounter >= FAILED_TRESHOLD {
					logrus.WithFields(logrus.Fields{
						"component":            s,
						"consecutive-failures": failedCounter,
					}).Panic("Too many consecutive failures")
				}
			} else {
				// Reset consecutive failure counter
				failedCounter = 0
			}

		}
	}
}

func (s *fileProducer) encode(stat aggregator.TimestampedStat) []byte {
	if s.configuration.Format == JSON {
		return util.JsonOf(stat)
	} else {
		return []byte(util.StringOf(stat) + "\n")
	}
}

package producer

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/natefinch/lumberjack"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/util"
)

type LoggingProducerConfiguration struct {
	formatConfiguration
	rollingFileConfiguration
	Name string
}

func (s LoggingProducerConfiguration) Create() Producer {
	s.validateFormat(s)

	logger := logrus.New()
	if s.Format == JSON {
		logger.Formatter = &logrus.JSONFormatter{}
	}
	logger.Out = &lumberjack.Logger{
		Filename:   s.FileName,
		MaxSize:    int(s.FileSize),
		MaxBackups: int(s.FileBackups),
		MaxAge:     int(s.FileAge),
	}

	return &loggingProducer{
		configuration: s,
		log:           logger,
	}
}

type loggingProducer struct {
	registeredProducer
	configuration LoggingProducerConfiguration
	log           *logrus.Logger
}

func (s *loggingProducer) Start(aggregator aggregator.ProducerAggregator) {
	s.register(aggregator)
	go s.produce(aggregator)
}

func (s *loggingProducer) Close() {
	s.registeredProducer.close()
}

func (s *loggingProducer) produce(aggregator aggregator.ProducerAggregator) {
	for {
		aggrStats := <-s.reg.Channel()
		stats := aggrStats.Stats()

		for i := 0; i < len(stats); i++ {
			s.log.WithFields(logrus.Fields{
				"vpp":         stats[i].VppUuid,
				"update-type": stats[i].StatType,
				"update":      s.encode(stats[i]),
			}).Info(fmt.Sprintf("VPP %v update detected", stats[i].StatType))
		}
	}
}

func (s *loggingProducer) encode(stat aggregator.TimestampedStat) interface{} {
	if s.configuration.Format == JSON {
		return stat
	} else {
		return util.StringOfNoType(stat)
	}
}

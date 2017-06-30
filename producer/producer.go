/*
Package producer provides producers that receive stats from aggregators.
A producer is the final component that forwards stats to external components.
*/
package producer

import (
	log "github.com/Sirupsen/logrus"
	"github.com/natefinch/lumberjack"
	"io"
	"pnda/vpp/monitoring/aggregator"
	"strings"
)

const TXT = "txt"
const JSON = "json"

const FAILED_TRESHOLD = 25

// Generic producer configuration(builder)
type ProducerConfiguration interface {
	Create() Producer
}

// Generic producer (receiving stats from aggregator and producing/forwarding them to external entities)
type Producer interface {
	Start(aggregator aggregator.ProducerAggregator)
	// FIXME replace with Closer interface from io!!! and others too
	Close()
}

// Base configuration for producers aware of a format setting
type formatConfiguration struct {
	Format string
}

func (s formatConfiguration) validateFormat(owner interface{}) {
	s.Format = strings.TrimSpace(s.Format)
	switch s.Format {
	case JSON:
	case TXT:
	default:
		log.WithFields(log.Fields{
			"component": owner,
			"format":    s.Format,
		}).Panic("Uncerognized format setting")
	}
}

// Base configuration for producers using some sort of rolling (file) writer
type rollingFileConfiguration struct {
	FileName    string
	FileSize    float64 // megabytes
	FileBackups float64
	FileAge     float64 // days
}

func (s rollingFileConfiguration) validateAndGetWriter(owner interface{}) io.WriteCloser {

	if s.FileName == "" {
		log.WithFields(log.Fields{
			"component": owner,
		}).Panic("Output file not set")
	}

	return &lumberjack.Logger{
		Filename:   s.FileName,
		MaxSize:    int(s.FileSize),
		MaxBackups: int(s.FileBackups),
		MaxAge:     int(s.FileAge),
	}
}

type registeredProducer struct {
	reg aggregator.ProducerRegistration
}

func (s *registeredProducer) register(aggregator aggregator.ProducerAggregator) {
	s.reg = aggregator.Register()
}

func (s *registeredProducer) close() {
	s.reg.Close()
}

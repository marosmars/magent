package producer

import (
	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/util"
	"reflect"
	"strconv"
	"time"
)

type KafkaProducerConfiguration struct {
	formatConfiguration
	Name    string
	Topic   string
	Brokers []string
}

func (s KafkaProducerConfiguration) Create() Producer {
	s.validateFormat(s)

	log.WithFields(log.Fields{
		"producer": s,
	}).Info("Initializing kafka connection")

	if internalProducer, err := sarama.NewAsyncProducer(s.Brokers, getKafkaConfig(s.Name)); err == nil {
		log.WithFields(log.Fields{
			"producer": s,
		}).Info("Kafka connection successful")

		return &kafkaProducer{
			configuration:    s,
			internalProducer: internalProducer,
		}
	} else {
		log.WithFields(log.Fields{
			"error": err,
		}).Panic("Unable to connect to kafka")
		panic("Unable to connect to kafka")
	}
}

func getKafkaConfig(name string) *sarama.Config {
	config := sarama.NewConfig()
	config.ClientID = name
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true

	return config
}

type kafkaProducer struct {
	registeredProducer
	configuration    KafkaProducerConfiguration
	internalProducer sarama.AsyncProducer
}

func (s *kafkaProducer) Start(aggregator aggregator.ProducerAggregator) {
	s.register(aggregator)
	go s.produce(aggregator)
}

func (s *kafkaProducer) Close() {
	s.registeredProducer.close()
}

func (s *kafkaProducer) produce(aggregator aggregator.ProducerAggregator) {
	failedCounter := 0

	for {
		aggrStats := <-s.reg.Channel()
		stats := aggrStats.Stats()

		for i := 0; i < len(stats); i++ {

			strTime := strconv.Itoa(int(time.Now().Unix()))
			msg := &sarama.ProducerMessage{
				Topic:    s.configuration.Topic,
				Key:      sarama.StringEncoder(strTime),
				Value:    s.encode(stats[i]),
				Metadata: reflect.TypeOf(stats[i]),
			}

			s.internalProducer.Input() <- msg

			select {
			case err := <-s.internalProducer.Errors():
				failedCounter++

				log.WithFields(log.Fields{
					"producer":             s.configuration,
					"error":                err,
					"consecutive-failures": failedCounter,
				}).Warn("Unable to send message to kafka")

				if failedCounter >= FAILED_TRESHOLD {
					log.WithFields(log.Fields{
						"component":            s,
						"consecutive-failures": failedCounter,
					}).Panic("Too many consecutive failures")
				}
			case <-s.internalProducer.Successes():
				log.WithFields(log.Fields{
					"message":  msg,
					"producer": s.configuration,
				}).Debug("Successfully sent message to kafka")
				failedCounter = 0
			}
		}
	}
}

func (s *kafkaProducer) encode(stat aggregator.TimestampedStat) sarama.Encoder {
	if s.configuration.Format == JSON {
		bytes := util.JsonOf(stat)
		return sarama.ByteEncoder(bytes)
	} else {
		return sarama.StringEncoder(util.StringOf(stat))
	}

}

package producer

import (
	"errors"
	"fmt"
	"github.com/Shopify/sarama/mocks"
	"github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/aggregator"
	"testing"
	"time"
)

func init() {
	logrus.SetLevel(logrus.PanicLevel)
}

func TestKafkaProducer(t *testing.T) {
	config := getKafkaConfig("testing")
	config.Producer.Return.Successes = true

	internalProducer := mocks.NewAsyncProducer(t, config)
	internalProducer.ExpectInputAndSucceed()

	prod := &kafkaProducer{
		configuration:    KafkaProducerConfiguration{Topic: "testing"},
		internalProducer: internalProducer,
	}
	defer prod.Close()

	ch := make(chan (aggregator.AggregatedStat))
	prod.Start(&testAggr{ch})

	content := "aa"
	ch <- testStats{stats: []aggregator.TimestampedStat{
		{
			Stat:      struct{ name string }{content},
			StatType:  "test",
			Timestamp: time.Now(),
			VppUuid:   aggregator.VppUuid("testVpp"),
		}}}
}

func TestKafkaProducerFailures(t *testing.T) {
	config := getKafkaConfig("testing")

	internalProducer := mocks.NewAsyncProducer(t, config)
	// mock the kafka producer, first stat fails, second succeeds and the rest fails to cause a panic
	internalProducer.ExpectInputAndFail(errors.New("Initial failure"))
	internalProducer.ExpectInputAndSucceed()
	for i := 0; i < FAILED_TRESHOLD; i++ {
		internalProducer.ExpectInputAndFail(fmt.Errorf("Failing item: %v", i))
	}

	prod := &kafkaProducer{
		configuration:    KafkaProducerConfiguration{Topic: "testing"},
		internalProducer: internalProducer,
	}
	defer prod.Close()
	ch := make(chan (aggregator.AggregatedStat))
	deferPanicChannel := make(chan (int))

	aggr := &testAggr{ch}
	i := 0

	// prod.start with defer
	go func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Invalid configuration succeeded without panic")
			}
			deferPanicChannel <- 0
		}()

		prod.register(aggr)
		prod.produce(aggr)
	}()

	// send in TRESHOLD + 2 stats to
	for ; i < FAILED_TRESHOLD+2; i++ {
		ch <- testStats{stats: []aggregator.TimestampedStat{
			{
				Stat:      struct{ name string }{string(i)},
				StatType:  "test",
				Timestamp: time.Now(),
				VppUuid:   aggregator.VppUuid("testVpp"),
			}}}
	}

	select {
	case <-deferPanicChannel:
	case <-time.After(time.Second * time.Duration(10)):
		t.Error("Timed out. Did not receive all stats")
	}
}

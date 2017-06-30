package producer

import (
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"pnda/vpp/monitoring/aggregator"
	"strings"
	"testing"
	"time"
)

func init() {
	logrus.SetLevel(logrus.ErrorLevel)
}

var validConfigs = []FileProducerConfiguration{
	{
		rollingFileConfiguration: rollingFileConfiguration{
			FileAge:     -78,
			FileName:    tempFile(),
			FileSize:    1,
			FileBackups: 1,
		},
		formatConfiguration: formatConfiguration{
			Format: "txt",
		},
		Name: "Test",
	},
}

func tempFile() string {
	if f, err := ioutil.TempFile("", "testingVppFileProducer"); err == nil {
		return f.Name()
	} else {
		panic(fmt.Errorf("Unable to create temp file, %v", err))
	}

}

type testStats struct {
	stats []aggregator.TimestampedStat
}

func (s testStats) Stats() []aggregator.TimestampedStat {
	return s.stats
}

type testReg struct {
	channel chan (aggregator.AggregatedStat)
}

func (s *testReg) Channel() chan (aggregator.AggregatedStat) {
	return s.channel
}
func (s *testReg) Close() {}

type testAggr struct {
	channel chan (aggregator.AggregatedStat)
}

func (s *testAggr) Register() aggregator.ProducerRegistration {
	return &testReg{channel: s.channel}
}

func TestCreateValidFile(t *testing.T) {
	for _, cfg := range validConfigs {

		prod := cfg.Create()
		if prod == nil {
			t.Error("Unable to create producer")
		}
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

		prod.Close()

		// Give the goroutine inside producer a chance to finish before we proceed to assertions
		time.Sleep(time.Second * time.Duration(5))

		fileContent, err := ioutil.ReadFile(cfg.FileName)
		if err != nil {
			t.Error("Unable to read file content")
		}

		fileString := string(fileContent)
		if !strings.Contains(fileString, content) {
			t.Errorf("File does not contain expected cotnent: %v, but is just: %v", content, fileString)
		}
	}
}

func TestCreateInvalidFile(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Invalid configuration succeeded without panic")
		}
	}()

	FileProducerConfiguration{
		rollingFileConfiguration: rollingFileConfiguration{
			FileAge:     1,
			FileName:    "",
			FileSize:    1,
			FileBackups: 1,
		},
		formatConfiguration: formatConfiguration{
			Format: "json",
		},
		Name: "Test",
	}.Create()
}

func TestCreateInvalidFormat(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Invalid configuration succeeded without panic")
		}
	}()

	FileProducerConfiguration{
		rollingFileConfiguration: rollingFileConfiguration{
			FileAge:     1,
			FileName:    tempFile(),
			FileSize:    1,
			FileBackups: 1,
		},
		formatConfiguration: formatConfiguration{
			Format: "NONEASdkjhasfd",
		},
		Name: "Test",
	}.Create()
}

type failingWriter struct {
}

func (s *failingWriter) Close() error {
	return nil
}
func (s *failingWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("Failing write")
}

func TestFileProducerFailures(t *testing.T) {
	prod := &fileProducer{
		configuration: FileProducerConfiguration{},
		writer:        &failingWriter{},
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

	// send in TRESHOLD
	for ; i < FAILED_TRESHOLD; i++ {
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

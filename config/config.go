/*
Package config provides vpp monitoring agent configuration capabilities.
*/
package config

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/util"
	"strings"
)

// Non parsed (only as map of maps) wiring and configuration imported from YAML format
type WiringInput struct {
	Collectors  map[string]interface{}
	Aggregators map[string]interface{}
	Producers   map[string]interface{}
}

const COLLECTOR_CFG_SUFFIX = "CollectorConfiguration"
const AGGREGATOR_CFG_SUFFIX = "AggregatorConfiguration"
const PRODUCER_CFG_SUFFIX = "ProducerConfiguration"

// Transform WiringInput into WiringConfiguration by parsing the content into a set of various components
func (s WiringInput) Parse() WiringConfiguration {
	log.WithField("wiring", util.StringOf(s)).Debug("Parsing wiring configuration")

	var collectors []CollectorWiring
	for name, configMap := range s.Collectors {
		collectorType := configMap.(map[string]interface{})["Type"].(string) + COLLECTOR_CFG_SUFFIX
		collectors = append(collectors,
			newCollectorConfiguration(collectorType, name, configMap.(map[string]interface{})))
	}

	var aggregators map[string]AggregatorWiring = make(map[string]AggregatorWiring)
	for name, configMap := range s.Aggregators {
		aggrType := configMap.(map[string]interface{})["Type"].(string) + AGGREGATOR_CFG_SUFFIX
		aggregators[name] =
			newAggregatorConfiguration(aggrType, name, configMap.(map[string]interface{}))
	}

	var producers []ProducerWiring
	for name, configMap := range s.Producers {
		prodType := configMap.(map[string]interface{})["Type"].(string) + PRODUCER_CFG_SUFFIX
		producers = append(producers,
			newProducerConfiguration(prodType, name, configMap.(map[string]interface{})))
	}

	wiringConfiguration := WiringConfiguration{
		Collectors:  collectors,
		Aggregators: aggregators,
		Producers:   producers,
	}

	log.WithField("wiring", util.StringOf(wiringConfiguration)).Debug("Wiring configuration parsed")
	return wiringConfiguration
}

// Parsed wiring and configuration for components to create and run
type WiringConfiguration struct {
	Collectors  []CollectorWiring
	Aggregators map[string]AggregatorWiring
	Producers   []ProducerWiring
}

// Holds parsed arguments to the agent
type Args struct {
	Profile     bool
	ProfilePort uint
	Debug       bool
	LogFile     string
	Wiring      WiringInput
	VppUuid     aggregator.VppUuid
}

// Parse input arguments for the agent
func ParseFlags() Args {
	profilePtr := flag.Bool("profile", false, "Enable profiling using pprof")
	profilePortPtr := flag.Uint("profile-port", 8080, "Port to expose profiloing information at")
	debugPtr := flag.Bool("debug", false, "Enable debug logging level")
	wiringFile := flag.String("wiring-file", "./configuration.yaml",
		"Specify the components, their wiring and configuration")
	logFile := flag.String("debug-log-file", "/tmp/vpp-monitoring-agent-debug.log",
		"Specify the output file for vpp-monitoring-agent debug logging")
	vppUuid := flag.String("vpp-uuid", "",
		"Specify a uinque ID of a monitored VPP. The ID will be added to the produced data. "+
			"Using 'hostid' utility can be one way of generating the UUID")

	flag.Parse()

	if *vppUuid == "" {
		log.Panic("VPP uuid argument has to be set. See the usage")
	}

	configContent, err := ioutil.ReadFile(*wiringFile)
	if err != nil {
		log.WithFields(log.Fields{
			"file":  *wiringFile,
			"error": err,
		}).Panic("Unable to read wiring configuration file")
	}

	var wiring WiringInput
	err = yaml.Unmarshal(configContent, &wiring)
	if err != nil {
		log.WithFields(log.Fields{
			"file":  *wiringFile,
			"error": err,
		}).Panic("Unable to parse wiring configuration file")
	}

	return Args{
		Profile:     *profilePtr,
		ProfilePort: *profilePortPtr,
		Debug:       *debugPtr,
		LogFile:     *logFile,
		VppUuid:     aggregator.VppUuid(fmt.Sprintf("vpp-%s", strings.TrimSpace(*vppUuid))),
		Wiring:      wiring,
	}
}

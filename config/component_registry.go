package config

import (
	"pnda/vpp/monitoring/collector"
	"pnda/vpp/monitoring/collector/ifc_counters"
	"pnda/vpp/monitoring/collector/ifc_info"
	"pnda/vpp/monitoring/collector/ifc_state"
	"pnda/vpp/monitoring/collector/version"

	log "github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/producer"
	"reflect"
)

var componentConfigTypeRegistry = make(map[string]reflect.Type)

func init() {
	// Add all available component types to the registry for further creation

	// Collectors
	addToRegistry(version.VersionCollectorConfiguration{})
	addToRegistry(ifc_counters.InterfaceCountersCollectorConfiguration{})
	addToRegistry(ifc_info.InterfaceInfoCollectorConfiguration{})
	addToRegistry(ifc_state.InterfaceStateChangesCollectorConfiguration{})

	// Aggregators
	addToRegistry(aggregator.BufferedAggregatorConfiguration{})
	addToRegistry(aggregator.FilteringAggregatorConfiguration{})

	// Producers
	addToRegistry(producer.LoggingProducerConfiguration{})
	addToRegistry(producer.FileProducerConfiguration{})
	addToRegistry(producer.KafkaProducerConfiguration{})

	log.WithField("components", componentConfigTypeRegistry).Info("Component registry initialized successfully")
}

func addToRegistry(instance interface{}) {
	t := reflect.TypeOf(instance)
	componentConfigTypeRegistry[t.String()] = t
}

type Scheduling struct {
	SchedulingType  string
	SchedulingDelay uint
}

// Wrapper structure for collector configuration with additional information e.g. requested scheduling
type CollectorWiring struct {
	Name       string
	Config     collector.CollectorConfiguration
	Scheduling Scheduling
	Aggregator string
}

const SCHEDULING_KEY = "Schedule"
const DELAY_KEY = "Delay"
const TYPE_KEY = "Type"
const AGGREGATOR_KEY = "Aggregator"
const NAME_KEY = "Name"
const CONFIGURATION_KEY = "Configuration"

func newCollectorConfiguration(clctrType string, name string, config map[string]interface{}) CollectorWiring {
	cfg := newConfiguration(clctrType, name, config)

	var scheduleDelay uint

	if _, isPresent := config[SCHEDULING_KEY].(map[string]interface{})[DELAY_KEY]; isPresent {
		scheduleDelay = uint(config[SCHEDULING_KEY].(map[string]interface{})[DELAY_KEY].(float64))
	}

	scheduling := Scheduling{
		SchedulingType:  config[SCHEDULING_KEY].(map[string]interface{})[TYPE_KEY].(string),
		SchedulingDelay: scheduleDelay,
	}

	return CollectorWiring{
		Name:       name,
		Config:     cfg.Interface().(collector.CollectorConfiguration),
		Scheduling: scheduling,
		Aggregator: config[AGGREGATOR_KEY].(string),
	}
}

// Wrapper structure for collector configuration with additional information e.g. requested scheduling
type AggregatorWiring struct {
	Name   string
	Config aggregator.AggregatorConfiguration
}

func newAggregatorConfiguration(aggrType string, name string, config map[string]interface{}) AggregatorWiring {
	cfg := newConfiguration(aggrType, name, config)

	return AggregatorWiring{
		Name:   name,
		Config: cfg.Interface().(aggregator.AggregatorConfiguration),
	}
}

// Wrapper structure for collector configuration with additional information e.g. requested scheduling
type ProducerWiring struct {
	Name       string
	Config     producer.ProducerConfiguration
	Aggregator string
}

func newProducerConfiguration(producerType string, name string, config map[string]interface{}) ProducerWiring {
	cfg := newConfiguration(producerType, name, config)

	return ProducerWiring{
		Name:       name,
		Config:     cfg.Interface().(producer.ProducerConfiguration),
		Aggregator: config["Aggregator"].(string),
	}
}

func newConfiguration(clctrType string, name string, config map[string]interface{}) reflect.Value {
	requestedType := componentConfigTypeRegistry[clctrType]

	if requestedType == nil {
		log.WithFields(log.Fields{
			"type":            clctrType,
			"available-types": componentConfigTypeRegistry,
		}).Panic("Unable to create instance")
	}

	log.WithFields(log.Fields{
		"component-type": clctrType,
		"component-name": name,
	}).Info("Creating component")
	cfg := reflect.New(requestedType).Elem()

	// Set name
	cfg.FieldByName(NAME_KEY).Set(reflect.ValueOf(name))

	if cfgValue, isPresent := config[CONFIGURATION_KEY]; isPresent && cfgValue != nil {

		for cfgKey, cfgValue := range config[CONFIGURATION_KEY].(map[string]interface{}) {
			// FIXME check like here http://stackoverflow.com/questions/6395076/in-golang-using-reflect-how-do-you-set-the-value-of-a-struct-field
			field := cfg.FieldByName(cfgKey)

			// FIXME this repairs the type of slices to always string, so only string lists from YAML will work
			if reflect.TypeOf(cfgValue).Kind() == reflect.Slice {
				var typed []string
				for _, i := range cfgValue.([]interface{}) {
					typed = append(typed, string(i.(string)))
				}
				cfgValue = typed
			}

			valueReflected := reflect.ValueOf(cfgValue)

			// FIXME all numbers are parsed as float64

			log.WithFields(log.Fields{
				"collector":        clctrType,
				"field-name":       cfgKey,
				"field-type":       field.Kind(),
				"field-value":      cfgValue,
				"field-value-type": valueReflected.Kind(),
			}).Debug("Setting component field")

			field.Set(valueReflected)
		}
	}

	return cfg
}

/*
Package monitoring implements a configurable standalone vpp monitoring agent.

The agent is implemented in pure Go and Cgo. It connects to VPP through its
shared memory APIs.
*/
package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/natefinch/lumberjack"
	"net/http"
	_ "net/http/pprof"
	"os"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/collector"
	"pnda/vpp/monitoring/collector/keepalive"
	"pnda/vpp/monitoring/config"
	"pnda/vpp/monitoring/govpp"
	"pnda/vpp/monitoring/util"
)

const CONNECTION_NAME = "vpp-monitoring-agent"

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func main() {
	args := config.ParseFlags()

	log.WithFields(log.Fields{
		"args": util.StringOfNoType(args),
	}).Info("Starting vpp-monitoring-agent")

	// Set the logging output
	log.SetOutput(&lumberjack.Logger{
		Filename:   args.LogFile,
		MaxSize:    10,
		MaxBackups: 2,
		MaxAge:     0,
	})

	if args.Debug {
		log.SetLevel(log.DebugLevel)
	}

	vppUuid := args.VppUuid

	wiringAndConfig := args.Wiring.Parse()

	// Process aggregator wiring and create instances
	var aggregatorMap = createAggregators(wiringAndConfig)
	// Process producers wiring and create instances
	createAndStartProducers(wiringAndConfig, aggregatorMap)

	// Start aggregators
	startAggregators(aggregatorMap, vppUuid)
	// TODO close started aggregators and producers

	if args.Profile {
		go startProfiling(args)
	}

	for {
		log.Info("Starting VPP monitoring agent")
		connAttempt := &govpp.VppConnectionAttempt{Name: CONNECTION_NAME}
		connection := connAttempt.Connect()

		keepaliveFailureCh := make(chan (int))
		keepaliveStopCh := make(chan (int))
		keepaliveExec := keepalive.KeepaliveCollectorConfiguration{
			Name:    "Keepalive-executor",
			Timeout: 10,
		}.Create(keepaliveFailureCh)
		collector.CollectScheduled(connection, keepaliveExec, 10, keepaliveStopCh)

		var collectorExecutionStopChannels [](chan (int))
		// Put the first stop channel (for keepalive) in
		collectorExecutionStopChannels = append(collectorExecutionStopChannels, keepaliveStopCh)
		var createdCollectors []collector.Collector

		for _, clctrWiringAndConfig := range wiringAndConfig.Collectors {
			clctr := clctrWiringAndConfig.Config.Create(aggregatorMap[clctrWiringAndConfig.Aggregator])
			createdCollectors = append(createdCollectors, clctr)

			stopCh := scheduleCollector(clctrWiringAndConfig, connection, clctr)
			if stopCh != nil {
				collectorExecutionStopChannels = append(collectorExecutionStopChannels, stopCh)
			}
		}

		// Block until a keepalive fails
		<-keepaliveFailureCh
		close(keepaliveFailureCh)

		log.Error("Keepalive failure detected, reconnecting VPP and reinitializing collectors")

		log.Info("Stopping all collector executions")
		for _, stopChannel := range collectorExecutionStopChannels {
			stopChannel <- -1
		}
		log.Info("Closing all collectors")
		for _, createdCollector := range createdCollectors {
			createdCollector.Close()
		}

		// This reconnect loop only works in theory, because with VPP disconnect and connect still crash the program
		// so an external loop is needed to restart the agent
		//connection.Disconnect()
		// so instead of disconnect and loop, just return -2 as an indication to the external loop
		os.Exit(100)
	}
}

func createAggregators(wiringAndConfig config.WiringConfiguration) map[string](aggregator.Aggregator) {
	var aggregatorMap map[string](aggregator.Aggregator) = make(map[string](aggregator.Aggregator))
	for name, aggrWiringAndConfig := range wiringAndConfig.Aggregators {
		aggr := aggrWiringAndConfig.Config.Create()
		aggregatorMap[name] = aggr
	}

	return aggregatorMap
}

func startAggregators(aggregatorMap map[string](aggregator.Aggregator), uuid aggregator.VppUuid) {
	for _, aggr := range aggregatorMap {
		aggr.Start(uuid)
	}
}

func createAndStartProducers(wiringAndConfig config.WiringConfiguration, aggregatorMap map[string](aggregator.Aggregator)) {
	for _, producerWiringAndConfig := range wiringAndConfig.Producers {
		prod := producerWiringAndConfig.Config.Create()
		prod.Start(aggregatorMap[producerWiringAndConfig.Aggregator])
	}
}

// Go to http://localhost:<debug-port>/debug/pprof/ to evaluate profiling results
func startProfiling(args config.Args) {
	log.Info("Exposing profiling information")
	http.ListenAndServe(fmt.Sprintf(":%v", args.ProfilePort), http.DefaultServeMux)
}

func scheduleCollector(clctrWiringAndConfig config.CollectorWiring, connection *govpp.VppConnection, clctr collector.Collector) chan (int) {
	switch clctrWiringAndConfig.Scheduling.SchedulingType {

	case collector.NOTIFICATION_SCHEDULING:
		fallthrough
	case collector.ONCE_SCHEDULING:
		collector.CollectOnce(connection, clctr)
		return nil
	case collector.REPEATED_SCHEDULING:
		if clctrWiringAndConfig.Scheduling.SchedulingDelay < 1 {
			log.WithFields(log.Fields{
				"delay":     clctrWiringAndConfig.Scheduling.SchedulingDelay,
				"component": clctrWiringAndConfig.Name,
			}).Panic("Invalid scheduling delay, needs to be >0")
		}
		stopCh := make(chan (int))
		collector.CollectScheduled(connection, clctr,
			clctrWiringAndConfig.Scheduling.SchedulingDelay, stopCh)
		return stopCh
	default:
		log.WithFields(log.Fields{
			"format":    clctrWiringAndConfig.Scheduling.SchedulingType,
			"component": clctrWiringAndConfig.Name,
		}).Panic("Uncerognized scheduling type setting")
		return nil
	}
}

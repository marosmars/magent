package collector

import (
	log "github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/govpp"
	"time"
)

type Collector interface {
	Collect(connection *govpp.VppConnection)
	Close()
}

type CollectorConfiguration interface {
	Create(aggregator aggregator.CollectorAggregator) Collector
}

// Notifications are the same as once, its here just for clarity
const NOTIFICATION_SCHEDULING = "notifications"
const ONCE_SCHEDULING = "once"
const REPEATED_SCHEDULING = "scheduled"

func CollectOnce(connection *govpp.VppConnection, clctr Collector) {
	log.WithFields(log.Fields{
		"collector": clctr,
	}).Info("Executing collector")

	defer func() {
		if r := recover(); r != nil {
			log.WithFields(log.Fields{
				"collector": clctr,
				"panic":     r,
			}).Error("Collector execution failed")
		}
	}()

	clctr.Collect(connection)
}

func CollectScheduled(connection *govpp.VppConnection, clctr Collector, delayInSeconds uint, stopChannel chan (int)) {
	go func() {

		log.WithFields(log.Fields{
			"collector":        clctr,
			"delay-in-seconds": delayInSeconds,
		}).Info("Executing scheduled collector at fixed rate")

	loop:
		for {
			CollectOnce(connection, clctr)

			select {
			case <-stopChannel:
				log.WithFields(log.Fields{
					"collector": clctr,
				}).Debug("Stopping exeuction")
				close(stopChannel)
				break loop

			case <-time.After(time.Second * time.Duration(delayInSeconds)):
				// Just a regular schedule
			}
		}
	}()
}

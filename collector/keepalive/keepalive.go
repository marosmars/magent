package keepalive

import (
	log "github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/collector"
	"pnda/vpp/monitoring/govpp"
	"time"
)

type KeepaliveCollectorConfiguration struct {
	Name    string
	Timeout float64
}

var singletonCollector *keepaliveExecutor = nil

func (s KeepaliveCollectorConfiguration) Create(keepaliveFailureChannel chan (int)) collector.Collector {
	if singletonCollector != nil {
		log.WithFields(log.Fields{
			"collector": singletonCollector,
		}).Panic("Collector(singleton) already exists")
	}

	singletonCollector = &keepaliveExecutor{
		configuration:           s,
		keepaliveFailureChannel: keepaliveFailureChannel,
	}

	log.WithFields(log.Fields{
		"collector": singletonCollector,
	}).Debug("KeepaliveCollector created successfully")

	return singletonCollector
}

type keepaliveExecutor struct {
	configuration           KeepaliveCollectorConfiguration
	keepaliveFailureChannel chan (int)
}

func (s keepaliveExecutor) Collect(connection *govpp.VppConnection) {
	var ch = make(chan (uint))

	ctx := connection.NextContextId()
	log.WithField("ctx", ctx).Debug("Executing keepalive ping")
	connection.Ping(ctx, func(_, ctx uint) {
		ch <- ctx
	})

	select {
	case <-ch:
		log.WithField("ctx", ctx).Debug("Keepalive executed successfully")
		close(ch)
		break
	case <-time.After(time.Second * time.Duration(s.configuration.Timeout)):
		log.WithField("ctx", ctx).Error("Keepalive failed")
		s.keepaliveFailureChannel <- -1
		close(ch)
		break
	}
}

func (s keepaliveExecutor) Close() {
	s.configuration = KeepaliveCollectorConfiguration{}
	singletonCollector = nil
}

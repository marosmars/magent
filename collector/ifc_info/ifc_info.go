package ifc_info

/*
#cgo LDFLAGS: -L/usr/lib/x86_64-linux-gnu -lvlibmemoryclient -lvlibapi -lsvm -lvppinfra -lpthread -lm -lrt -lpneum

#include <vnet/vnet.h>
#include <vlib/vlib.h>
#include <vlib/unix/unix.h>
#include <vlibapi/api.h>
#include <vlibmemory/api.h>

#include <vpp/api/vpe_msg_enum.h>

#define vl_typedefs
#include <vpp/api/vpe_all_api_h.h>
#undef vl_typedefs

#define vl_endianfun
#include <vpp/api/vpe_all_api_h.h>
#undef vl_endianfun

// Callback cannot be in Go, since it's impossible to send a Go func pointer to C
void detailsCallback(u32 context, u32 index, char * name, u32 l2AddrLength, char * l2Address);
static inline void vl_api_sw_interface_details_t_handler(vl_api_sw_interface_details_t * mp) {
	detailsCallback(clib_net_to_host_u32(mp->context), clib_net_to_host_u32(mp->sw_if_index), mp->interface_name,
			clib_net_to_host_u32(mp->l2_address_length), mp->l2_address);
}

static inline void register_callback() {
	vl_msg_api_set_handlers(VL_API_SW_INTERFACE_DETAILS, "sw_interface_details",
	vl_api_sw_interface_details_t_handler,
	vl_noop_handler, vl_noop_handler, vl_noop_handler,
	sizeof(vl_api_sw_interface_details_t), 1);
}

static inline vl_api_sw_interface_dump_t* new_request(u32 client_id, u32 context) {
	vl_api_sw_interface_dump_t * mp;
	mp = vl_msg_api_alloc(sizeof(*mp));
	memset (mp, 0, sizeof (*mp));

	mp->_vl_msg_id = ntohs (VL_API_SW_INTERFACE_DUMP);
	mp->client_index = client_id;
	mp->context = context;
	return mp;
}
*/
import "C"
import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"net"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/collector"
	"pnda/vpp/monitoring/govpp"
	"pnda/vpp/monitoring/util"
	"sync"
	"unsafe"
)

type InterfaceInfoCollectorConfiguration struct {
	Name string
	// TODO configure which fields to include in the report
}

// Support for multiple instances
var collectorInstancesLock sync.Mutex
var collectorInstances = make(map[InterfaceInfoCollectorConfiguration](interfaceInfoCollector))
var callbackRegistered = false

func (s InterfaceInfoCollectorConfiguration) Create(aggregator aggregator.CollectorAggregator) collector.Collector {
	if !callbackRegistered {
		C.register_callback()
	}

	if _, ok := collectorInstances[s]; ok {
		log.WithFields(log.Fields{
			"configuration": s,
		}).Panic("Collector already exists with same configuration")
	}

	clctr := interfaceInfoCollector{
		configuration: s,
		mapOfChannels: make(map[uint](chan networkInterface)),
		aggregator:    aggregator,
	}

	collectorInstancesLock.Lock()
	collectorInstances[s] = clctr
	collectorInstancesLock.Unlock()

	log.WithFields(log.Fields{
		"collector-instances": collectorInstances,
	}).Debug("InterfaceInfoCollector created successfully")

	return clctr
}

type interfaceInfoCollector struct {
	configuration InterfaceInfoCollectorConfiguration
	// Support for concurrent execution of Collect
	mapOfChannels map[uint](chan networkInterface)
	aggregator    aggregator.CollectorAggregator
}

type networkInterface struct {
	InterfaceName  string `json:"interface_name"`
	InterfaceIndex uint   `json:"interface_index"`
	L2Address      string `json:"l2_address"`
}

type interfaces struct {
	Interfaces []networkInterface `json:"interfaces"`
}

func (s interfaceInfoCollector) Close() {
	if clctr, isPresent := collectorInstances[s.configuration]; isPresent {
		for _, ch := range clctr.mapOfChannels {
			close(ch)
		}
	}
	delete(collectorInstances, s.configuration)
}

func (s interfaceInfoCollector) Collect(connection *govpp.VppConnection) {
	ctxId := connection.NextContextId()

	s.mapOfChannels[ctxId] = make(chan networkInterface)

	go collectInfo(s.aggregator, s.mapOfChannels[ctxId])

	connection.SendMessage(
		unsafe.Pointer(
			C.new_request(
				C.u32(connection.ClientIndex),
				C.clib_host_to_net_u32(C.u32(ctxId)))))

	connection.Ping(connection.NextContextId(), func(pid uint, ctx uint) {
		close(s.mapOfChannels[ctxId])
		delete(s.mapOfChannels, ctxId)
	})
}

//export detailsCallback
func detailsCallback(context C.u32, idx C.u32, name *C.char, l2AddrLength C.u32, l2Address *C.char) {
	var l2Addr string
	if uint(l2AddrLength) > 0 {
		goData := C.GoBytes(unsafe.Pointer(l2Address), C.int(l2AddrLength))
		l2Addr = net.HardwareAddr(goData).String()
	}

	info := networkInterface{C.GoString(name), uint(idx), l2Addr}

	log.WithFields(log.Fields{
		"interface-details": util.StringOf(info),
	}).Debug("Received interface details")

	findChannelByContext(uint(context)) <- info
}

func findChannelByContext(context uint) chan networkInterface {
	collectorInstancesLock.Lock()
	defer collectorInstancesLock.Unlock()

	for _, clctr := range collectorInstances {
		for ctxId, channel := range clctr.mapOfChannels {
			if ctxId == context {
				return channel
			}
		}
	}

	panic(fmt.Errorf("Unable to find channel under contextId: %v", context))
}

func collectInfo(aggregator aggregator.CollectorAggregator, ch chan networkInterface) {
	var allInfos []networkInterface

	for {
		info, ok := <-ch
		if !ok {
			// Channel closed
			break
		}
		allInfos = append(allInfos, info)
	}

	aggregatedInfos := interfaces{Interfaces: allInfos}

	log.WithFields(log.Fields{
		"aggregated-interface-details": util.StringOf(aggregatedInfos),
	}).Debug("Aggregated interface details")

	aggregator.Channel() <- aggregatedInfos
}

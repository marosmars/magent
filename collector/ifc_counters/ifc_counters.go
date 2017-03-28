package ifc_counters

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

// Callback cannot be in Go, since it's impossible to send pointer to a Go func to C
void wantStatsCallback(u32 retval);
static inline void vl_api_want_stats_reply_t_handler(vl_api_want_stats_reply_t * mp) {
	wantStatsCallback(clib_net_to_host_u32(mp->retval));
}

void ifcCounterCallback(u8 type, u32 ifc_index, u32 count, u8 * data);
void ifcCombinedCounterCallback(u8 type, u32 ifc_index, u32 count, u8 * data);
static inline void vl_api_vnet_interface_counters_t_handler(vl_api_vnet_interface_counters_t * mp) {

	if (mp->is_combined == 0) {
		ifcCounterCallback(mp->vnet_counter_type, clib_net_to_host_u32(mp->first_sw_if_index),
			clib_net_to_host_u32(mp->count), mp->data);
	} else {
		ifcCombinedCounterCallback(mp->vnet_counter_type, clib_net_to_host_u32(mp->first_sw_if_index),
			clib_net_to_host_u32(mp->count), mp->data);
	}
}

static inline void register_callback() {
	vl_msg_api_set_handlers(VL_API_WANT_INTERFACE_EVENTS_REPLY, "want_stats_reply",
	vl_api_want_stats_reply_t_handler,
	vl_noop_handler, vl_noop_handler, vl_noop_handler,
	sizeof(vl_api_want_stats_reply_t), 1);

	vl_msg_api_set_handlers(VL_API_VNET_INTERFACE_COUNTERS, "vnet_interface_counters",
	vl_api_vnet_interface_counters_t_handler,
	vl_noop_handler, vl_noop_handler, vl_noop_handler,
	sizeof(vl_api_vnet_interface_counters_t), 1);

	// FIXME handle ipv4_fib_counters
	//vl_msg_api_set_handlers(VL_API_VNET_IP4_FIB_COUNTERS, "vnet_ip4_fib_counters",
	//vl_api_vnet_ip4_fib_counters_t_handler,
	//vl_noop_handler, vl_noop_handler, vl_noop_handler,
	//sizeof(vl_api_vnet_ip4_fib_counters_t), 1);
}

static inline vl_api_want_stats_t* new_request(u32 client_id, u32 enable) {
	vl_api_want_stats_t * mp;
	mp = vl_msg_api_alloc(sizeof(*mp));
	memset (mp, 0, sizeof (*mp));

	mp->_vl_msg_id = ntohs (VL_API_WANT_STATS);
	mp->client_index = client_id;
	mp->enable_disable = enable;
	return mp;
}
*/
import "C"
import (
	"bytes"
	"encoding/binary"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/collector"
	"pnda/vpp/monitoring/govpp"
	"pnda/vpp/monitoring/util"
	"unsafe"
)

type InterfaceCountersCollectorConfiguration struct {
	Name string
}

var singletonCollector *interfaceCountersCollector = nil

func (s InterfaceCountersCollectorConfiguration) Create(aggregator aggregator.CollectorAggregator) collector.Collector {
	if singletonCollector != nil {
		log.WithFields(log.Fields{
			"collector": singletonCollector,
		}).Panic("Collector(singleton) already exists")
	}

	// FIXME handle ipv4_fib_counters
	C.register_callback()

	singletonCollector = &interfaceCountersCollector{
		configuration: s,
		aggregator:    aggregator,
	}

	log.WithFields(log.Fields{
		"collector": singletonCollector,
	}).Debug("InterfaceCountersCollector created successfully")

	return singletonCollector
}

type interfaceCountersCollector struct {
	configuration InterfaceCountersCollectorConfiguration
	aggregator    aggregator.CollectorAggregator
}

type interfaceCounter interface{}

// Counters

type CounterType byte

const (
	DROP CounterType = iota
	PUNT
	IP4
	IP6
	RX_NOBUF
	RX_MISS
	RX_ERROR
	TX_ERROR
	MPLS
	SIMPLE_INTERFACE_COUNTER
)

type counter struct {
	InterfaceIndex uint   `json:"interface_index"`
	PacketCount    uint64 `json:"packet_count"`
}

type dropCounters struct {
	Counters []interfaceCounter `json:"drop_counters"`
}

type ip4Counters struct {
	Counters []interfaceCounter `json:"ipv4_counters"`
}

type ip6Counters struct {
	Counters []interfaceCounter `json:"ipv6_counters"`
}

// Combined counters

type combinedCounter struct {
	InterfaceIndex uint   `json:"interface_index"`
	PacketCount    uint64 `json:"packet_count"`
	ByteCount      uint64 `json:"byte_count"`
}

type rxCombinedCounters struct {
	Counters []interfaceCounter `json:"rx_counters"`
}

type txCombinedCounters struct {
	Counters []interfaceCounter `json:"tx_counters"`
}

type CombinedCounterType byte

const (
	RX CombinedCounterType = iota
	TX
)

//export wantStatsCallback
func wantStatsCallback(retval C.u32) {
	if retval < 0 {
		log.WithField("retval", retval).Panic("Interface counters activation failed")
	}

	log.WithFields(log.Fields{
		"retval": retval,
	}).Debug("Successfully activated interface counter notifications")
}

//export ifcCounterCallback
func ifcCounterCallback(counterType C.u8, ifcIndex C.u32, count C.u32, data *C.u8) {
	var result []interfaceCounter
	var ctrType CounterType

	// 8 == size of uint64, 2 == entry for packages + entry for bytes, count == number of interfaces
	goData := C.GoBytes(unsafe.Pointer(data), C.int(8*2*count))
	buf := bytes.NewReader(goData)

	// DATA is in shape of:
	// [ifc0-packetCount, ifc1-packetCount, 0, 0]
	// in case of non-combined, the byte counters are always 0
	// in case of combined, they are filled in as well

	for i := uint(ifcIndex); i < uint(count); i++ {
		var pktsCounter uint64

		//binary.Read(buf, binary.BigEndian, &bytesCounter)
		binary.Read(buf, binary.BigEndian, &pktsCounter)

		if pktsCounter == 0 {
			// Skip 0-ed counters
			continue
		}

		ctrType = CounterType(counterType)
		result = append(result, counter{InterfaceIndex: i, PacketCount: pktsCounter})
	}

	if len(result) == 0 {
		// Skip empty counters
		return
	}

	if aggrCounter, err := wrapCounters(result, ctrType); err == nil {
		log.WithFields(log.Fields{
			"interface-counter": util.StringOf(aggrCounter),
		}).Debug("Received ifc counter notifications")
		singletonCollector.aggregator.Channel() <- aggrCounter
	}
}

func wrapCounters(result []interfaceCounter, ctrType CounterType) (aggregator.Stat, error) {
	switch ctrType {
	case DROP:
		return dropCounters{Counters: result}, nil
	case IP4:
		return ip4Counters{Counters: result}, nil
	case IP6:
		return ip6Counters{Counters: result}, nil
	default:
		return nil, fmt.Errorf("Unsupported couter type: %v", ctrType)
	}
}

//export ifcCombinedCounterCallback
func ifcCombinedCounterCallback(counterType C.u8, ifcIndex C.u32, count C.u32, data *C.u8) {
	var result []interfaceCounter
	var ctrType CombinedCounterType

	// 8 == size of uint64, 2 == entry for packages + entry for bytes, count == number of interfaces
	goData := C.GoBytes(unsafe.Pointer(data), C.int(8*2*count))
	buf := bytes.NewReader(goData)

	// DATA is in shape of:
	// [ifc0-packetCount, ifc0-byteCount, ifc1-packetCount, ifc1-byteCount]
	// in case of non-combined, the byte counters are always 0
	// in case of combined, they are filled in as well

	for i := uint(ifcIndex); i < uint(count); i++ {
		var pktsCounter, bytesCounter uint64

		binary.Read(buf, binary.BigEndian, &pktsCounter)
		binary.Read(buf, binary.BigEndian, &bytesCounter)

		if pktsCounter == 0 && bytesCounter == 0 {
			// Skip 0-ed counters
			continue
		}

		ctrType = CombinedCounterType(counterType)
		result = append(result,
			combinedCounter{InterfaceIndex: i, PacketCount: pktsCounter, ByteCount: bytesCounter})
	}

	if len(result) == 0 {
		// Skip empty counters
		return
	}

	if aggrCounter, err := wrapCombinedCounters(result, ctrType); err == nil {
		log.WithFields(log.Fields{
			"interface-counter": util.StringOf(aggrCounter),
		}).Debug("Received ifc combined counter notifications")
		singletonCollector.aggregator.Channel() <- aggrCounter
	}
}

func wrapCombinedCounters(result []interfaceCounter, ctrType CombinedCounterType) (aggregator.Stat, error) {
	switch ctrType {
	case RX:
		return rxCombinedCounters{result}, nil
	case TX:
		return txCombinedCounters{result}, nil
	default:
		return nil, fmt.Errorf("Unsupported combined couter type: %v", ctrType)
	}
}

func (s interfaceCountersCollector) Collect(connection *govpp.VppConnection) {
	C.register_callback()

	connection.SendMessage(
		unsafe.Pointer(
			C.new_request(
				C.u32(connection.ClientIndex),
				C.clib_host_to_net_u32(C.u32(1)))))
}

func (s interfaceCountersCollector) Close() {
	s.aggregator = nil
	s.configuration = InterfaceCountersCollectorConfiguration{}
	singletonCollector = nil
}

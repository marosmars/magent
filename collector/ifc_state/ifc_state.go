package ifc_state

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
void wantEventsCallback(u32 retval);
static inline void vl_api_want_interface_events_reply_t_handler(vl_api_want_interface_events_reply_t * mp) {
	wantEventsCallback(clib_net_to_host_u32(mp->retval));
}

void ifcStateChangeCallback(u32 index, u8 admin, u8 link, u8 deleted);
static inline void vl_api_sw_interface_set_flags_t_handler(vl_api_sw_interface_set_flags_t * mp) {
	ifcStateChangeCallback(clib_net_to_host_u32(mp->sw_if_index), mp->admin_up_down, mp->link_up_down, mp->deleted);
}

static inline void register_callback() {
	vl_msg_api_set_handlers(VL_API_WANT_INTERFACE_EVENTS_REPLY, "want_interface_events_reply",
	vl_api_want_interface_events_reply_t_handler,
	vl_noop_handler, vl_noop_handler, vl_noop_handler,
	sizeof(vl_api_want_interface_events_reply_t), 1);

	vl_msg_api_set_handlers(VL_API_SW_INTERFACE_SET_FLAGS, "sw_interface_set_flags",
	vl_api_sw_interface_set_flags_t_handler,
	vl_noop_handler, vl_noop_handler, vl_noop_handler,
	sizeof(vl_api_sw_interface_set_flags_t), 1);
}

static inline vl_api_want_interface_events_t* new_request(u32 client_id, u32 enable, u32 pid) {
	vl_api_want_interface_events_t * mp;
	mp = vl_msg_api_alloc(sizeof(*mp));
	memset (mp, 0, sizeof (*mp));

	mp->_vl_msg_id = ntohs (VL_API_WANT_INTERFACE_EVENTS);
	mp->client_index = client_id;
	mp->enable_disable = enable;
	mp->pid = pid;
	return mp;
}
*/
import "C"
import (
	log "github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/collector"
	"pnda/vpp/monitoring/govpp"
	"pnda/vpp/monitoring/util"
	"unsafe"
)

type InterfaceStateChangesCollectorConfiguration struct {
	Name string
}

var singletonCollector *interfaceStateCollector = nil

func (s InterfaceStateChangesCollectorConfiguration) Create(aggregator aggregator.CollectorAggregator) collector.Collector {
	if singletonCollector != nil {
		log.WithFields(log.Fields{
			"collector": singletonCollector,
		}).Panic("Collector(singleton) already exists")
	}

	C.register_callback()

	singletonCollector = &interfaceStateCollector{
		configuration: s,
		aggregator:    aggregator,
	}

	log.WithFields(log.Fields{
		"collector": singletonCollector,
	}).Debug("InterfaceStateCollector created successfully")

	return singletonCollector
}

type interfaceStateCollector struct {
	configuration InterfaceStateChangesCollectorConfiguration
	aggregator    aggregator.CollectorAggregator
}

type interfaceStateChange struct {
	InterfaceIndex uint `json:"interface_index"`
	AdminState     bool `json:"admin_state"`
	LinkState      bool `json:"link_state"`
}

type interfaceDeleted struct {
	InterfaceIndex uint `json:"interface_index"`
}

//export wantEventsCallback
func wantEventsCallback(retval C.u32) {
	if retval < 0 {
		log.WithField("retval", retval).Panic("Interface events activation failed")
	}

	log.WithFields(log.Fields{
		"retval": retval,
	}).Debug("Successfully activated interface state notifications")
}

//export ifcStateChangeCallback
func ifcStateChangeCallback(index C.u32, admin C.u8, link C.u8, deleted C.u8) {
	var ifcStateChange interface{}
	if byte(deleted) == 1 {
		ifcStateChange = interfaceDeleted{uint(index)}
	} else {
		ifcStateChange = interfaceStateChange{uint(index), admin != 0, link != 0}
	}

	log.WithFields(log.Fields{
		"interface-state-update": util.StringOf(ifcStateChange),
	}).Debug("Received ifc state change notification")

	singletonCollector.aggregator.Channel() <- ifcStateChange
}

func (s interfaceStateCollector) Collect(connection *govpp.VppConnection) {
	C.register_callback()

	connection.SendMessage(
		unsafe.Pointer(
			C.new_request(
				C.u32(connection.ClientIndex),
				C.clib_host_to_net_u32(C.u32(1)),
				C.clib_host_to_net_u32(C.u32(connection.Pid)))))
}

func (s interfaceStateCollector) Close() {
	s.aggregator = nil
	s.configuration = InterfaceStateChangesCollectorConfiguration{}
	singletonCollector = nil
}

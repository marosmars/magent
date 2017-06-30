package version

/*
#cgo LDFLAGS: -L/usr/lib/x86_64-linux-gnu -lvlibmemoryclient -lvlibapi -lsvm -lvppinfra -lpthread -lm -lrt -lpneum

#include <vnet/vnet.h>
#include <vlib/vlib.h>
#include <vlib/unix/unix.h>
#include <vlibapi/api.h>
#include <vlibmemory/api.h>

#include <vpp-api/vpe_msg_enum.h>

#define vl_typedefs
#include <vpp-api/vpe_all_api_h.h>
#undef vl_typedefs

#define vl_endianfun
#include <vpp-api/vpe_all_api_h.h>
#undef vl_endianfun

// Callback cannot be in Go, since it's impossible to send a Go func pointer to C
void versionCallback(i32 retval, char * program, char * version, char * dir, char * date);
static inline void vl_api_show_version_reply_t_handler(vl_api_show_version_reply_t * mp) {
	versionCallback(clib_net_to_host_u32(mp->retval), mp->program, mp->version, mp->build_directory, mp->build_date);
}

static inline void register_callback() {
	vl_msg_api_set_handlers(VL_API_SHOW_VERSION_REPLY, "show_version_reply",
	vl_api_show_version_reply_t_handler,
	vl_noop_handler, vl_noop_handler, vl_noop_handler,
	sizeof(vl_api_show_version_reply_t), 1);
}

static inline vl_api_show_version_t* new_request(u32 client_id) {
	vl_api_show_version_t * mp;
	mp = vl_msg_api_alloc(sizeof(*mp));
	memset (mp, 0, sizeof (*mp));

	mp->_vl_msg_id = ntohs (VL_API_SHOW_VERSION);
	mp->client_index = client_id;
	return mp;
}
*/
import "C"
import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"pnda/vpp/monitoring/aggregator"
	"pnda/vpp/monitoring/collector"
	"pnda/vpp/monitoring/govpp"
	"pnda/vpp/monitoring/util"
	"unsafe"
)

type VersionCollectorConfiguration struct {
	Name string
}

var singletonCollector *versionCollector = nil

func (s VersionCollectorConfiguration) Create(aggregator aggregator.CollectorAggregator) collector.Collector {
	if singletonCollector != nil {
		log.WithFields(log.Fields{
			"collector": singletonCollector,
		}).Panic("Collector(singleton) already exists")
	}

	C.register_callback()

	singletonCollector = &versionCollector{
		configuration: s,
		aggregator:    aggregator,
	}

	log.WithFields(log.Fields{
		"collector": singletonCollector,
	}).Debug("VersionCollector created successfully")

	return singletonCollector
}

type versionCollector struct {
	configuration VersionCollectorConfiguration
	aggregator    aggregator.CollectorAggregator
}

type version struct {
	Program        string `json:"program"`
	Version        string `json:"version"`
	BuildDirectory string `json:"build_directory"`
	BuildDate      string `json:"build_date"`
}

func (s version) String() string {
	return fmt.Sprintf("%#v", s)
}

//export versionCallback
func versionCallback(retval C.i32, program *C.char, vppVersion *C.char, buildDir *C.char, buildDate *C.char) {
	if retval < 0 {
		log.WithField("retval", retval).Panic("Interface events activation failed")
	}

	info := version{C.GoString(program), C.GoString(vppVersion), C.GoString(buildDir), C.GoString(buildDate)}

	log.WithFields(log.Fields{
		"version": util.StringOf(info),
	}).Debug("Version details polled successfully")

	singletonCollector.aggregator.Channel() <- info
}

func (s *versionCollector) Collect(connection *govpp.VppConnection) {
	connection.SendMessage(
		unsafe.Pointer(
			C.new_request(
				C.u32(connection.ClientIndex))))
}

func (s *versionCollector) Close() {
	s.aggregator = nil
	s.configuration = VersionCollectorConfiguration{}
	singletonCollector = nil
}

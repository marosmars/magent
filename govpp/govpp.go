package govpp

/*
#cgo LDFLAGS: -L/usr/lib/x86_64-linux-gnu -lvlibmemoryclient -lvlibapi -lsvm -lvppinfra -lpthread -lm -lrt -lpneum

#include <stdlib.h>

#include <pneum/pneum.h>
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
*/
import "C"
import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"reflect"
	"sync"
	"unsafe"
)

type VppConnectionAttempt struct {
	Name string
}

func (attempt VppConnectionAttempt) Connect() *VppConnection {
	log.WithFields(log.Fields{
		"configuration": attempt,
	}).Debug("Attempting connect to VPP APIs")

	// FIXME add a watchdog goroutine to time out connection (and try infinitely) ... it's infinite

	cs := C.CString("/vpe-api")
	defer C.free(unsafe.Pointer(cs))
	csName := C.CString(attempt.Name)
	defer C.free(unsafe.Pointer(csName))

	if rv := C.vl_client_connect_to_vlib(cs, csName, 32); rv < 0 {
		log.WithFields(log.Fields{
			"return": rv,
		}).Panic("Unable to open a connection to VPP APIs")
	}

	apiMain := reflect.ValueOf(C.api_main)
	shmemHdr := apiMain.FieldByName("shmem_hdr")
	queue := reflect.Indirect(shmemHdr).FieldByName("vl_input_queue")

	connection := VppConnection{
		apiMain:     apiMain,
		queue:       unsafe.Pointer(reflect.Indirect(queue).UnsafeAddr()),
		ClientIndex: uint(apiMain.FieldByName("my_client_index").Int()),
		contextId:   0,
	}

	connection.Pid, _ = controlPingSync(&connection, 0)

	log.WithFields(log.Fields{
		"pid": connection.Pid,
	}).Debug("Successfully invoked initial ping")

	log.WithFields(log.Fields{
		"connection": connection.String(),
	}).Info("Successfully connected to VPP APIs")

	return &connection
}

type VppConnection struct {
	ClientIndex uint
	Pid         uint
	Lock        sync.Mutex
	apiMain     reflect.Value
	queue       unsafe.Pointer
	contextId   uint
}

func (s *VppConnection) String() string {
	return fmt.Sprintf("{VPP API CONNECTION: Pid:%v, ClientId: %v, Queue:%v, ClientIndex:%v}",
		s.Pid, s.ClientIndex, s.queue, s.ClientIndex)
}

func (s *VppConnection) Disconnect() {
	log.Debug("Attempting disconnecting from VPP APIs")

	C.vl_client_disconnect_from_vlib()
	s.ClientIndex = 0
	s.Pid = 0
	s.Lock = sync.Mutex{}
	s.apiMain = reflect.ValueOf(nil)
	s.queue = nil
	s.contextId = 0

	log.Debug("VPP APIs disconnected successfully")
}

func (s *VppConnection) SendMessage(msg unsafe.Pointer) {
	C.vl_msg_api_send_shmem((*C.struct__unix_shared_memory_queue)(s.queue), (*C.u8)(unsafe.Pointer(&msg)))
}

// Blocking invocation of a control ping
func (s *VppConnection) Ping(ctx uint, callback func(pid uint, ctx uint)) {
	controlPing(s, ctx, callback)
}

func (s *VppConnection) NextContextId() uint {
	return s.Locked(func() interface{} {
		log.WithFields(log.Fields{
			"current-context": s.contextId,
			"next-context":    s.contextId + 1,
		}).Debug("Getting next context ID")

		s.contextId = s.contextId + 1
		return s.contextId
	}).(uint)
}

func (s *VppConnection) Locked(lambda func() interface{}) interface{} {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	return lambda()
}

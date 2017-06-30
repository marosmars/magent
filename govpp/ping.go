package govpp

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

void pingCallback(u32 retval, u32 pid, u32 ctx);
static inline void vl_api_control_ping_reply_t_handler(vl_api_control_ping_reply_t * mp) {
	pingCallback(clib_net_to_host_u32(mp->retval),
			clib_net_to_host_u32(mp->vpe_pid),
			clib_net_to_host_u32(mp->context));
}

static inline void register_callback() {
	vl_msg_api_set_handlers(VL_API_CONTROL_PING_REPLY, "control_ping_reply",
	vl_api_control_ping_reply_t_handler, vl_noop_handler, vl_noop_handler, vl_noop_handler,
	sizeof(vl_api_control_ping_reply_t), 1);
}

static inline vl_api_control_ping_t* new_request(u32 client_id, u32 context) {
	vl_api_control_ping_t * mp;
	mp = vl_msg_api_alloc(sizeof(*mp));
	memset (mp, 0, sizeof (*mp));

	mp->_vl_msg_id = ntohs (VL_API_CONTROL_PING);
	mp->client_index = client_id;
	mp->context = context;
	return mp;
}
*/
import "C"
import (
	log "github.com/Sirupsen/logrus"
	"sync"
	"unsafe"
)

var callbacks = make(map[uint](func(pid uint, ctx uint)))
var callbacksLock sync.Mutex

func init() {
	C.register_callback()
}

//export pingCallback
func pingCallback(retval C.u32, pid C.u32, ctx C.u32) {
	log.WithFields(log.Fields{
		"retval": retval,
		"ctx":    ctx,
	}).Debug("Pinged successfully")

	if retval < 0 {
		log.WithField("retval", retval).Panic("Control ping failed")
	}

	uintCtx := uint(ctx)

	callbacksLock.Lock()
	defer callbacksLock.Unlock()

	callback := callbacks[uintCtx]
	if callback == nil {
		log.WithFields(log.Fields{
			"callbacks": callbacks,
			"ctx":       uintCtx,
		}).Panic("Cannot find control ping callback")
	}
	defer delete(callbacks, uintCtx)

	callback(uint(pid), uintCtx)
}

func controlPing(connection *VppConnection, ctx uint, callback func(pid uint, ctx uint)) {
	log.WithField("ctx", ctx).Debug("Invoking control ping")

	callbacksLock.Lock()
	defer callbacksLock.Unlock()
	callbacks[ctx] = callback

	connection.SendMessage(
		unsafe.Pointer(
			C.new_request(
				C.u32(connection.ClientIndex),
				C.clib_host_to_net_u32(C.u32(ctx)),
			)))

}

func controlPingSync(connection *VppConnection, ctx uint) (pid uint, returnContext uint) {
	tempCh := make(chan uint)
	// Destroy the channel
	defer func() {
		close(tempCh)
	}()

	controlPing(connection, ctx, func(pid uint, returnContext uint) {
		tempCh <- pid
		tempCh <- returnContext
	})

	return <-tempCh, <-tempCh
}

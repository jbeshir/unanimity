package chrequest

import (
	"sync"
	"time"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/chrequest/chproto"
	"github.com/jbeshir/unanimity/shared/store"
)

type requestTimeout struct {
	request  *chproto.ChangeRequest
	duration time.Duration
	timer    *time.Timer
	canceled bool
}

type forwardTimeout struct {
	forward  *chproto.ChangeForward
	timer    *time.Timer
	canceled bool
}

type requestKey struct {
	requestNode uint16
	requestId   uint64
}

var requestTimeoutLock sync.Mutex
var requestTimeouts map[requestKey]*requestTimeout
var forwardTimeouts map[requestKey]*forwardTimeout

var requestTimedOut chan *requestTimeout
var forwardTimedOut chan *forwardTimeout

func init() {
	requestTimedOut = make(chan *requestTimeout, 100)
	forwardTimedOut = make(chan *forwardTimeout, 100)
}

// Must not be called concurrently with any other code touching timeouts,
// or with reads from the timeout channels.
func addRequestTimeout(request *chproto.ChangeRequest, duration time.Duration) {
	timeout := new(requestTimeout)
	timeout.request = request
	timeout.duration = duration
	timeout.timer = time.AfterFunc(duration,
		func() { requestTimedOut <- timeout })

	var key requestKey
	key.requestNode = uint16(*request.RequestNode)
	key.requestId = *request.RequestId

	if _, exists := requestTimeouts[key]; exists {
		panic("tried to add duplicate request timeout")
	}
	requestTimeouts[key] = timeout
}

// Must not be called concurrently with any other code touching timeouts,
// or with reads from the timeout channels.
func getRequestTimeout(requestNode uint16, requestId uint64) (*requestTimeout,
	bool) {

	var key requestKey
	key.requestNode = requestNode
	key.requestId = requestId

	v, exists := requestTimeouts[key]
	return v, exists
}

// Must not be called concurrently with any other code touching timeouts,
// or with reads from the timeout channels.
func cancelRequestTimeout(requestNode uint16, requestId uint64) {
	var key requestKey
	key.requestNode = requestNode
	key.requestId = requestId

	if v, exists := requestTimeouts[key]; exists {
		v.canceled = true
		delete(requestTimeouts, key)
	}
}

// Must not be called concurrently with any other code touching timeouts,
// or with reads from the timeout channels.
func addForwardTimeout(forward *chproto.ChangeForward) {
	timeout := new(forwardTimeout)
	timeout.forward = forward
	timeout.timer = time.AfterFunc(config.ROUND_TRIP_TIMEOUT_PERIOD,
		func() { forwardTimedOut <- timeout })

	var key requestKey
	key.requestNode = uint16(*forward.Request.RequestNode)
	key.requestId = *forward.Request.RequestId

	if _, exists := forwardTimeouts[key]; exists {
		panic("tried to add duplicate forward timeout")
	}
	forwardTimeouts[key] = timeout
}

// Must not be called concurrently with any other code touching timeouts,
// or with reads from the timeout channels.
func getForwardTimeout(requestNode uint16, requestId uint64) (*forwardTimeout,
	bool) {

	var key requestKey
	key.requestNode = requestNode
	key.requestId = requestId

	v, exists := forwardTimeouts[key]
	return v, exists
}

// Must not be called concurrently with any other code touching timeouts,
// or with reads from the timeout channels.
func cancelForwardTimeout(requestNode uint16, requestId uint64) {

	var key requestKey
	key.requestNode = requestNode
	key.requestId = requestId

	if v, exists := forwardTimeouts[key]; exists {
		v.canceled = true
		delete(forwardTimeouts, key)
	}
}

func handleChosenRequests(slot uint64) {

	relativeSlot := int(slot - store.InstructionStart())
	instruction := store.InstructionSlots()[relativeSlot][0]
	chrequest := instruction.ChangeRequest()

	requestTimeoutLock.Lock()

	cancelRequestTimeout(chrequest.RequestNode, chrequest.RequestId)

	requestTimeoutLock.Unlock()
}

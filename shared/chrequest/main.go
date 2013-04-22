/*
	Package implementing the Change Request Protocol,
	listening for and creating incoming and outgoing
	Change Request Protocol connections,
	and managing receiving and forwarding change requests.

	Uses shared/store’s callback registration functions to
	react to newly added instructions.

	Provides a startup function, methods to create new change requests
	and a change callback to receive change requests that this node is
	the leader for, applicable only to core nodes.
*/
package chrequest

import (
	"github.com/jbeshir/unanimity/shared/chrequest/chproto"
	"github.com/jbeshir/unanimity/shared/connect"
	"github.com/jbeshir/unanimity/shared/store"
)

type receivedConn struct {
	node uint16
	conn *connect.BaseConn
}

type chosenInstruction struct {
	slot     uint64
	waitChan chan bool
}

// Must be unbuffered.
// This ensures that we block until the processing goroutine has received
// the message, which ensures any terminate message arrives after the
// received message.
var receivedConnCh chan receivedConn

var newRequestCh chan *chproto.ChangeRequest

func init() {
	// Unbuffered.
	receivedConnCh = make(chan receivedConn)

	newRequestCh = make(chan *chproto.ChangeRequest, 100)

	store.AddChosenCallback(handleChosenRequests)
}

func Startup() {

	// Start processing goroutine.
	go process()

	// Start accepting change request connections.
	go connect.Listen(connect.CHANGE_REQUEST_PROTOCOL, incomingConn)
}

// Request a new change based on the given change request.
func Request(request *store.ChangeRequest) {

	// Create internal request based on the store request.
	internalRequest := new(chproto.ChangeRequest)
	internalRequest.RequestEntity = new(uint64)
	internalRequest.RequestNode = new(uint32)
	internalRequest.RequestId = new(uint64)
	*internalRequest.RequestEntity = request.RequestEntity
	*internalRequest.RequestNode = uint32(request.RequestNode)
	*internalRequest.RequestId = request.RequestId
	internalRequest.Changeset = make([]*chproto.Change,
		len(request.Changeset))

	for i := range request.Changeset {
		internalChange := new(chproto.Change)
		internalChange.TargetEntity = new(uint64)
		internalChange.Key = new(string)
		internalChange.Value = new(string)
		*internalChange.TargetEntity = request.Changeset[i].TargetEntity
		*internalChange.Key = request.Changeset[i].Key
		*internalChange.Value = request.Changeset[i].Value
		internalRequest.Changeset[i] = internalChange
	}

	newRequestCh <- internalRequest
}

func incomingConn(node uint16, conn *connect.BaseConn) {

	receivedConnCh <- receivedConn{node: node, conn: conn}
	handleConn(node, conn)
}

package connect

import (
	"net"
	"sync"
)

import (
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/shared/connect/baseproto"
)

// Our supported set of capabilities.
// Always empty in this implementation.
var capabilities []string

type BaseConn struct {

	// May only be written to/closed from the connection's write goroutine.
	// May only be read from from the connection's read goroutine.
	conn net.Conn

	// May only be sent to by the connection's write goroutine.
	disconnected chan struct{}

	// May only be read from by the connection's write goroutine.
	send chan *baseproto.Message
	stop chan struct{} // Requests clean termination.

	// May only be sent to from the connection's read goroutine,
	// and read from by the connection's write goroutine.
	receivedCapabilities chan *baseproto.Message

	// May only be sent to from the connection's read goroutine.
	received chan *baseproto.Message

	// Used so that Close() only closes the stop channel once.
	closeOnce sync.Once

	// Messages read from the base connection are written to this.
	// Read from this to receive messages from the connection.
	// Closed when the connection is terminated.
	Received <-chan *baseproto.Message

	// Closed when the connection is disconnected.
	// Exists in addition to Received in order to allow non-readers
	// to recognise connection termination.
	Disconnected <-chan struct{}
}

// Creates a BaseConn wrapping the given net.Conn.
func newBaseConn(conn net.Conn) *BaseConn {

	b := new(BaseConn)
	b.conn = conn

	b.disconnected = make(chan struct{})
	b.send = make(chan *baseproto.Message, 5)
	b.stop = make(chan struct{})
	b.received = make(chan *baseproto.Message, 5)

	b.Received = b.received
	b.Disconnected = b.disconnected

	go b.readLoop()
	go b.writeLoop()

	return b
}

// Read from a base connection in a loop.
// The top-level function for the BaseConn's read goroutine.
func (b *BaseConn) readLoop() {

	msgBuffer := make([]byte, 1024)
	var readLen uint64
	var length int
	var readUpTo int
	for {
		// If we're still reading the length...
		if length == 0 {

			// Read up to a maximum of six bytes.
			// Six bytes encodes up to 2^(7*6) == 2^35,
			// much more than the legal maximum length.
			// We don't want to read too much, because we have to
			// copy any excess read down the buffer afterwards.
			if readUpTo < 6 {
				n, err := b.conn.Read(msgBuffer[readUpTo:6])
				if err != nil {
					break
				}
				readUpTo += n
			}

			// Try to read the length.
			var used int
			readLen, used = proto.DecodeVarint(msgBuffer[:readUpTo])
			if used == 0 {
				// If we couldn't read it yet,
				// and have already read six bytes,
				// the other end is playing silly buggers.
				// Drop the connection.
				if readUpTo >= 6 {
					break
				}

				// Otherwise, if we weren't able to read it,
				// continue reading the length.
				continue
			}

			// If readLen is illegally huge, drop connection.
			// Otherwise, we've got our length.
			if readLen > 0x7FFFFFFF {
				break
			}
			length = int(readLen)

			// Grow message buffer if needed.
			if length > len(msgBuffer) {
				newMsgBuffer := make([]byte, length)

				// Copy over any excess read.
				if used != readUpTo {
					copy(newMsgBuffer,
						msgBuffer[used:readUpTo])
				}

				msgBuffer = newMsgBuffer
			} else {
				// Copy down any excess read.
				if used != readUpTo {
					copy(msgBuffer,
						msgBuffer[used:readUpTo])
				}
			}

			// This leaves readUpTo set to the length of
			// any excess read, zero if there was none.
			readUpTo -= used
		}

		// Read the message.
		// We don't want to read too much, because we have to
		// copy any excess read down the buffer afterwards.
		// It can still happen with short messages already read past.
		if readUpTo < length {
			n, err := b.conn.Read(msgBuffer[readUpTo:length])
			if err != nil {
				break
			}
			readUpTo += n

			continue
		}

		// Unmarshal the message and send it for receipt.
		msg := new(baseproto.Message)
		err := proto.Unmarshal(msgBuffer[:length], msg)
		if err != nil {
			break
		}
		if *msg.MsgType == 1 {
			b.receivedCapabilities <- msg
		} else {
			b.received <- msg
		}

		// Copy down any excess read.
		if length != readUpTo {
			copy(msgBuffer, msgBuffer[length:readUpTo])
		}

		// This leaves readUpTo set to the length of
		// any excess read, zero if there was none.
		readUpTo -= length

		// Set length to 0, ready to read the next length.
		length = 0
	}

	close(b.received)
	b.Close()
}

// Arbitrates attempts to write to the connection, and handles negotiation.
// The top-level function for the BaseConn's write goroutine.
func (b *BaseConn) writeLoop() {

	b.writeCapabilities()

	var negotiationDone bool
	var waitingMsgs []*baseproto.Message

mainloop:
	for {
		if !negotiationDone {
			select {
			case <-b.stop:
				// The stop channel closing indicates a close.
				break mainloop

			case msg := <-b.send:
				// Put messages to send into the waiting queue.
				waitingMsgs = append(waitingMsgs, msg)

			case msg := <-b.receivedCapabilities:

				// Unmarshal the capabilities message.
				// This node has an empty capabilities set,
				// so we always leave the connection's
				// capabilities empty.
				capMsg := new(baseproto.Capabilities)
				err := proto.Unmarshal(msg.Content, capMsg)
				if err != nil {
					break mainloop
				}
				negotiationDone = true

				// Send waiting messages.
				for _, msg := range waitingMsgs {
					if err := b.writeMsg(msg); err != nil {
						break mainloop
					}
				}
				waitingMsgs = nil
			}
		} else {

			select {
			case <-b.stop:
				// The stop channel closing indicates a close.
				break mainloop

			case msg := <-b.send:
				// Send requested messages.
				err := b.writeMsg(msg)
				if err != nil {
					break mainloop
				}
			}
		}
	}

	// Kills the connection and receiver, if it hasn't died already.
	b.conn.Close()

	// Tell code using this connection that it is dead.
	// Cancels any currently blocked attempts to send to the connection.
	close(b.disconnected)
}

// Writes our initial capabilities message to the underlying connection.
func (b *BaseConn) writeCapabilities() error {

	capMsg := new(baseproto.Capabilities)

	msg := new(baseproto.Message)
	msg.MsgType = new(uint32)
	*msg.MsgType = 1

	var err error
	if msg.Content, err = proto.Marshal(capMsg); err != nil {
		return err
	}

	return b.writeMsg(msg)
}

// Writes a message to the underlying connection.
func (b *BaseConn) writeMsg(msg *baseproto.Message) error {
	msgBuffer, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	prefix := proto.EncodeVarint(uint64(len(msgBuffer)))
	_, err = b.conn.Write(prefix)
	if err != nil {
		return err
	}

	_, err = b.conn.Write(msgBuffer)
	return err
}

// Send a message to the connection.
// May safely be called multiple times concurrently.
func (b *BaseConn) Send(msg *baseproto.Message) {
	select {
	case <-b.Disconnected:
		// This base connection has shut down.
		return

	case b.send <- msg:
		// Message given to write goroutine to send.
		return
	}
}

// Serialises the given protobuf message as the content of a new message,
// with the given message type, and sends that message on the connection,
// via Send(). Closes the connection if msg won't serialise.
// Convenience method.
func (b *BaseConn) SendProto(msgType uint32, msg proto.Message) {

	content, err := proto.Marshal(msg)
	if err != nil {
		b.Close()
		return
	}

	baseMsg := new(baseproto.Message)
	baseMsg.MsgType = new(uint32)
	*baseMsg.MsgType = msgType
	baseMsg.Content = content

	b.Send(baseMsg)
}

// Close the connection.
// May safely be called more than once, and concurrently with Send()
func (b *BaseConn) Close() {
	b.closeOnce.Do(func() { close(b.stop) })
}

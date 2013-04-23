package relay

var receivedCallbacks []func(userMsg *UserMessage)

// Adds a new callback to be called when a relayed user message is received.
// May only be called prior to calling Startup(), usually in init().
// A callback may be called more than once concurrently.
// May be called after the node becomes degraded,
// if we have a backlog of received messages.
func AddReceivedCallback(cb func(userMsg *UserMessage)) {
	receivedCallbacks = append(receivedCallbacks, cb)
}

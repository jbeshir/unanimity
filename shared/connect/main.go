/*
	Package handling listening for and establishing authenticated,
	encrypted base protocol connections to another node for use for a
	specific higher level protocol on request.

	Uses TLS, and verifies TLS certificates.

	Wraps the established bytestream connections with the base protocol,
	and handles connection negotiation.

	Provides methods to write messages to the connection,
	and a channel to read received messages from.
*/
package connect

const (
	CHANGE_REQUEST_PROTOCOL = iota
	CLIENT_PROTOCOL
	CONSENSUS_PROTOCOL
	FOLLOW_PROTOCOL
	RELAY_PROTOCOL
)

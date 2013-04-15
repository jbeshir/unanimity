package connect

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"strconv"
)

import (
	"github.com/jbeshir/unanimity/config"
)

// Makes an outgoing connection using that protocol type to the given node ID.
// Returns a non-nil error if it is unable to connect.
// Panics if it is called with protocol set to CLIENT_PROTOCOL.
func Dial(protocol int, id uint16) (*BaseConn, error) {

	if protocol == CLIENT_PROTOCOL {
		panic("tried to make outgoing client protocol connection")
	}

	ip := config.NodeIP(id)
	ipStr := ip.String()
	port := getProtocolPort(protocol)
	portStr := strconv.FormatInt(int64(port), 10)

	tlsConfig := new(tls.Config)
	tlsConfig.Certificates = []tls.Certificate{*config.Certificate()}
	tlsConfig.RootCAs = config.NodeCertPool(id)

	tlsConn, err := tls.Dial("tcp", ipStr+":"+portStr, tlsConfig)
	if err != nil {
		return nil, err
	}

	return newBaseConn(tlsConn), nil
}

// Blocks listening for connections of that given protocol type,
// and calls the specified handler when one is received,
// passing the node ID of the connecting node, or 0 for the Client Protocol.
func Listen(protocol int, handler func(id uint16, conn *BaseConn)) {

	ip := config.NodeIP(config.Id())
	ipStr := ip.String()
	port := getProtocolPort(protocol)
	portStr := strconv.FormatInt(int64(port), 10)

	tlsConfig := new(tls.Config)
	tlsConfig.Certificates = []tls.Certificate{*config.Certificate()}
	if protocol != CLIENT_PROTOCOL {
		tlsConfig.ClientAuth = tls.RequireAnyClientCert
	}

	listener, err := tls.Listen("tcp", ipStr+":"+portStr, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		tlsConn := conn.(*tls.Conn)

		err = tlsConn.Handshake()
		if err != nil {
			tlsConn.Close()
			continue
		}

		if protocol != CLIENT_PROTOCOL {
			// Check this connecting node has authenticated.
			state := tlsConn.ConnectionState()
			if len(state.PeerCertificates) == 0 {
				tlsConn.Close()
				continue
			}
			cert := state.PeerCertificates[0]

			// Identify which node they authenticated as.
			matched := false
			for _, node := range config.Nodes() {
				var verifyOpts x509.VerifyOptions
				verifyOpts.Intermediates = new(x509.CertPool)
				verifyOpts.Roots = config.NodeCertPool(node)
				chains, err := cert.Verify(verifyOpts)
				if err != nil {
					continue
				}

				// Matched the node. Call our handler.
				if len(chains) > 0 {
					matched = true
					handler(node, newBaseConn(tlsConn))
					break
				}
			}

			// No matching node found. Close the connection.
			if !matched {
				tlsConn.Close()
			}
		}
	}
}

// Return the port we use for the given protocol.
func getProtocolPort(protocol int) int {
	switch protocol {
	case CONSENSUS_PROTOCOL:
		return 2705
	case CHANGE_REQUEST_PROTOCOL:
		return 2706
	case FOLLOW_PROTOCOL:
		return 2707
	case RELAY_PROTOCOL:
		return 2708
	case CLIENT_PROTOCOL:
		return 2709
	default:
		panic("illegal protocol")
	}
}

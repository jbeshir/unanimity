package config

import (
	"crypto/x509"
	"net"
)

type node struct {
	id   uint16
	ip   net.IP
	cert *x509.CertPool
}

var nodes = make(map[uint16]*node)
var coreNodeIds []uint16
var clientNodeIds []uint16
var allNodeIds []uint16

// Adds a node with the given ID to the configuration.
// Must be called only at startup.
// Whether the other node is a core or client node is inferred
// from the ID number.
func AddNode(id uint16, ip net.IP, cert *x509.CertPool) {
	if id == 0 {
		panic("config: tried to add node with an ID of 0")
	}

	nodes[id] = &node{id: id, ip: ip, cert: cert}
	allNodeIds = append(allNodeIds, id)
	if id <= 8192 {
		coreNodeIds = append(coreNodeIds, id)
	} else {
		clientNodeIds = append(clientNodeIds, id)
	}
}

// Returns a slice of node IDs.
// The returned slice is not copied for each call, and so must not be modified.
func Nodes() []uint16 {
	return allNodeIds
}

// Returns a slice of core node IDs.
// The returned slice is not copied for each call, and so must not be modified.
func CoreNodes() []uint16 {
	return coreNodeIds
}

// Returns a slice of client node IDs.
// The returned slice is not copied for each call, and so must not be modified.
func ClientNodes() []uint16 {
	return clientNodeIds
}

// Returns the IP address associated with the given node ID.
// Will panic if the node ID was not added via AddNode.
func NodeIP(id uint16) net.IP {
	return nodes[id].ip
}

// Returns the certificate pool containing the given node ID’s certificate.
// Will panic if the node ID was not added via AddNode.
func NodeCertPool(id uint16) *x509.CertPool {
	return nodes[id].cert
}

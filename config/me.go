package config

import (
	"crypto/tls"
	"log"
)

var me uint16
var myCert *tls.Certificate

// Sets the node’s ID. Must be called once and only once at startup.
// The node must already have been added using AddNode.
func SetId(id uint16) {
	if _, ok := nodes[id]; !ok {
		log.Fatal("config: tried to set our ID to an unknown node ID")
	}

	me = id
}

// Returns this node's ID number.
// Returns 0 if it has not been set yet.
func Id() uint16 {
	return me
}

// Returns whether this node is a core node.
// Inferred from this node's ID number.
func IsCore() bool {
	return me <= 8192
}

// Sets the node’s TLS certificate, including private key.
// Must be called once at startup.
func SetCertificate(cert *tls.Certificate) {
	if cert == nil {
		log.Fatal("config: tried to set our certificate to nil")
	}

	myCert = cert
}

// Returns this node's TLS certificate with attached private key.
// Returns nil if it has not been set yet.
func Certificate() *tls.Certificate {
	return myCert
}

package store

import (
	"github.com/jbeshir/unanimity/config"
)

var nodeFirstUnapplied = make(map[uint16]uint64)

// Sets the first unapplied slot on the given node.
func SetNodeFirstUnapplied(node uint16, slot uint64) {
	if !config.IsCore() {
		panic("non-core node tracking first unapplied of other nodes")
	}

	nodeFirstUnapplied[node] = slot
}

// Returns the first unapplied slot as far as we know for the given node.
// Returns 0 if we know of no higher value.
func NodeFirstUnapplied(node uint16) uint64 {
	return nodeFirstUnapplied[node]
}

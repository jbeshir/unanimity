/*
	Package providing a startup function initialising
	all client node functionality.

	Calls the shared package’s startup function.
*/
package client

import (
	"github.com/jbeshir/unanimity/client/listener"
	"github.com/jbeshir/unanimity/client/relay"
	"github.com/jbeshir/unanimity/shared"
)

func Startup() {

	// Startup shared functionality.
	shared.Startup()

	// Startup handling of relay protocol connections.
	relay.Startup()

	// Startup handling of client protocol connections.
	listener.Startup()
}

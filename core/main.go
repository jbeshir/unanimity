/*
	Package providing a startup function initialising
	all core node functionality.

	Calls the shared package’s startup function.
*/
package core

import (
	"github.com/jbeshir/unanimity/core/consensus"
	"github.com/jbeshir/unanimity/shared"
)

func Startup() {

	// Startup shared functionality.
	shared.Startup()

	// Start consensus protocol handling.
	consensus.Startup()
}

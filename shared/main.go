/*
	Package providing a startup function initialising
	all shared functionality.
*/
package shared

import (
	"github.com/jbeshir/unanimity/shared/chrequest"
	"github.com/jbeshir/unanimity/shared/store"
)

func Startup() {
	// Initialise datastore.
	store.Startup()

	// Initialise change request and follow protocol handling.
	chrequest.Startup()
}

/*
	Package providing functions to set and retrieve the node’s provided
	configuration. Contains configuration for both core and client nodes.
	A given node may not use all configuration values.

	Includes TLS client certificates for authenticating other nodes,
	and TLS authentication data for authenticating to other nodes.

	Also provides constants, such as the two timeout periods.

	Must be initialised on startup.
*/
package config

import (
	"time"
)

// The timeout period allowed for round trip communications, in nanoseconds.
// This is an estimated reasonable default for most circumstances.
const ROUND_TRIP_TIMEOUT_PERIOD time.Duration = 5 * time.Second

// The timeout period allowed for a full change to occur, in nanoseconds.
// This may take up to six communication delays, or three round trips.
const CHANGE_TIMEOUT_PERIOD time.Duration = 3 * ROUND_TRIP_TIMEOUT_PERIOD

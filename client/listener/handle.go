package listener

import (
	"code.google.com/p/go.crypto/scrypt"
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/client/listener/cliproto_down"
	"github.com/jbeshir/unanimity/client/listener/cliproto_up"
	"github.com/jbeshir/unanimity/shared/store"
)

// Main function for the handling goroutine for conn.
func handleConn(conn *userConn) {

	// Do cleanup in a defer, so if they crash us we still tidy up,
	// at least here.
	defer func() {
		// Remove the connection from our connection set if present.
		// It will not be present if we are degraded or similar.
		store.StartTransaction()
		defer store.EndTransaction()
		if _, exists := connections[conn]; exists {
			delete(connections, conn)
		}
	}()

	for {
		msg, ok := <-conn.conn.Received
		if !ok {
			break
		}

		switch *msg.MsgType {
		case 2:
			handleAuth(conn, msg.Content)
		default:
			conn.conn.Close()
		}
	}

}

// Can only be called from the handling goroutine for conn.
func handleAuth(conn *userConn, content []byte) {
	var msg cliproto_up.Authenticate
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.conn.Close()
		return
	}

	if conn.session != 0 || conn.waitingAuth != nil {
		conn.conn.Close()
		return
	}

	store.StartTransaction()
	defer store.EndTransaction()

	userId := store.NameLookup("user", "name username", *msg.Username)
	if userId != 0 {
		if *msg.Password == "" {
			sendAuthFail(conn, "Invalid Password")
			return
		}

		// The user already exists.
		user := store.GetEntity(userId)

		// Try to authenticate them to it.
		salt := []byte(user.Value("salt"))
		pass := []byte(*msg.Password)
		key, err := scrypt.Key(pass, salt, 16384, 8, 1, 32)
		if err != nil {
			sendAuthFail(conn, "Invalid Password")
			return
		}
		if user.Value("password") != string(key) {
			sendAuthFail(conn, "Invalid Password")
			return
		}

		// It's the real user.
		if *msg.SessionId != 0 {
			// They are attaching to an existing session.
		} else {
			// They are creating a new session.
		}
	} else {
		// The user does not already exist.
		// Perform registration.
	}
}

func sendAuthFail(conn *userConn, reason string) {
	var msg cliproto_down.AuthenticationFailed
	msg.Reason = new(string)
	*msg.Reason = reason
	
	conn.conn.SendProto(2, &msg)
}

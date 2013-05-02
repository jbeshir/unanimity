package listener

import (
	"crypto/rand"
	"strconv"
	"strings"
)

import (
	"code.google.com/p/go.crypto/scrypt"
	"code.google.com/p/goprotobuf/proto"
)

import (
	"github.com/jbeshir/unanimity/client/listener/cliproto_down"
	"github.com/jbeshir/unanimity/client/listener/cliproto_up"
	"github.com/jbeshir/unanimity/client/relay"
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/chrequest"
	"github.com/jbeshir/unanimity/shared/store"
)

// Main function for the handling goroutine for conn.
func handleConn(conn *userConn) {

	// Do cleanup in a defer, so if they crash us we still tidy up here.
	// A small nod towards tolerating bad input, with much more needed.
	defer func() {
		// Remove the connection from our connection set if present.
		// It will not be present if we are degraded or similar.
		connectionsLock.Lock()

		if _, exists := connections[conn]; exists {
			delete(connections, conn)
		}

		// Remove the connection from our waiting map if present.
		// This must happen before session removal, as otherwise
		// auth could complete between the session check and this one.
		waitingLock.Lock()
		if conn.waitingAuth != nil {
			waitingConn := waiting[conn.waitingAuth.requestId]
			if waitingConn == conn {
				delete(waiting, conn.waitingAuth.requestId)
			}
		}
		waitingLock.Unlock()

		// Remove the connection from our sessions map if present.
		// It will not be present if we are degraded,
		// or have replaced this connection.
		sessionsLock.Lock()
		if conn.session != 0 {
			sessionConn := sessions[conn.session]
			if sessionConn == conn {
				delete(sessions, conn.session)
			}
		}
		sessionsLock.Unlock()


		connectionsLock.Unlock()
	}()

	for {
		select {
		case msg, ok := <-conn.conn.Received:
			if !ok {
				break
			}

			switch *msg.MsgType {
			case 2:
				handleAuth(conn, msg.Content)
			case 3:
				handleFollowUsername(conn, msg.Content)
			case 4:
				handleFollowUser(conn, msg.Content)
			case 5:
				handleStopFollowingUser(conn, msg.Content)
			case 6:
				handleSend(conn, msg.Content)
			default:
				conn.conn.Close()
			}

		case userMsg := <-conn.deliver:

			// Get the sender's user ID.
			store.StartTransaction()
			attachedTo := store.AllAttachedTo(userMsg.Sender)
			store.EndTransaction()

			// We don't know who the sender is. Drop message.
			if len(attachedTo) == 0 {
				break
			}

			// Deliver any user messages given to us.
			var deliverMsg cliproto_down.Received
			deliverMsg.SenderUserId = new(uint64)
			deliverMsg.SenderSessionId = new(uint64)
			deliverMsg.Tag = new(string)
			deliverMsg.Content = new(string)
			*deliverMsg.SenderUserId = attachedTo[0]
			*deliverMsg.SenderSessionId = userMsg.Sender
			*deliverMsg.Tag = userMsg.Tag
			*deliverMsg.Content = userMsg.Content

			conn.conn.SendProto(10, &deliverMsg)
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

	// TODO: Validate username and password against constraints.

	// We hold this through quite a lot of logic,
	// including password hashing, which is useful to parallelise.
	// TODO: Don't block the whole node for so long here.
	store.StartTransaction()
	defer store.EndTransaction()
	sessionsLock.Lock()
	defer sessionsLock.Unlock()
	waitingLock.Lock()
	defer waitingLock.Unlock()
	
	if conn.session != 0 || conn.waitingAuth != nil {
		conn.conn.Close()
		return
	}

	userId := store.NameLookup("user", "name username", *msg.Username)
	if userId != 0 {
		if *msg.Password == "" {
			sendAuthFail(conn, "Invalid Password")
			return
		}

		// The user already exists.
		user := store.GetEntity(userId)

		// Try to authenticate them to it.
		salt := []byte(user.Value("auth salt"))
		pass := []byte(*msg.Password)
		key, err := scrypt.Key(pass, salt, 16384, 8, 1, 32)
		if err != nil {
			sendAuthFail(conn, "Invalid Password")
			return
		}
		if user.Value("auth password") != string(key) {
			sendAuthFail(conn, "Invalid Password")
			return
		}

		// It's the real user.
		if *msg.SessionId != 0 {

			// They are attaching to an existing session.
			// Check it exists and is attached to this user.
			strSessionId := strconv.FormatUint(*msg.SessionId, 10)
			if user.Value("attach "+strSessionId) == "" {
				sendAuthFail(conn, "Invalid Session")
				return
			}

			// The session does exist.
			conn.session = *msg.SessionId

			// If this node is already attached to
			// the session, drop the other connection.
			if sessions[conn.session] != nil {
				sessions[conn.session].conn.Close()
			} else {
				// Create change attaching this node ID
				// to the session.
				id := config.Id()
				idStr := strconv.FormatUint(uint64(id), 10)

				chset := make([]store.Change, 1)
				chset[0].TargetEntity = conn.session
				chset[0].Key = "attach " + idStr
				chset[0].Value = "true"
				req := makeRequest(chset)
				go chrequest.Request(req)
			}

			// Put us in the sessions map.
			sessions[conn.session] = conn

			// Tell the client they authenticated successfully.
			sendAuthSuccess(conn, "")
		} else {
			// They are creating a new session.
			req := makeNewSessionRequest(userId)
			go chrequest.Request(req)

			// Stuff details in waiting auth.
			conn.waitingAuth = new(authData)
			conn.waitingAuth.msg = msg
			conn.waitingAuth.requestId = req.RequestId
			waiting[conn.waitingAuth.requestId] = conn
		}
	} else {
		// The user does not already exist.
		// Check they weren't trying to attach to a session.
		if *msg.SessionId != 0 {
			sendAuthFail(conn, "User Does Not Exist")
			return
		}

		// We're creating a new user.
		newUser := *msg.Username
		newPass := *msg.Password

		if !strings.HasPrefix(newUser, "Guest-") {

			// We're creating a new non-guest user.
			// Make sure they have a password.
			if newPass == "" {
				sendAuthFail(conn, "No Password")
				return
			}

			// Get a cryptographically random salt.
			saltBytes := make([]byte, 32)
			read := 0
			for read != len(saltBytes) {
				n, err := rand.Read(saltBytes[read:])
				read += n
				if err != nil {
					// Out of random?
					// TODO: Unsure when this can happen.
					sendAuthFail(conn, "Try Later")
					return
				}
			}

			// Generate their salted, hashed password.
			passBytes := []byte(*msg.Password)
			key, err := scrypt.Key(passBytes, saltBytes, 16384,
				8, 1, 32)
			if err != nil {
				// If you can get here, it's probably
				// because of a bad password.
				sendAuthFail(conn, "Invalid Password")
				return
			}
			hash := string(key)
			salt := string(saltBytes)
			
			// Create the new user.
			req := makeNewUserRequest(newUser, hash, salt, false)
			go chrequest.Request(req)

			// Stuff details in waiting auth.
			conn.waitingAuth = new(authData)
			conn.waitingAuth.msg = msg
			conn.waitingAuth.requestId = req.RequestId
			waiting[conn.waitingAuth.requestId] = conn

			return
		}

		// We're creating a new guest user.
		// Guests get automatic passwords, and can't set them.
		if newPass != "" {
			sendAuthFail(conn, "Cannot Set Password For Guest User")
			return
		}

		// Get a cryptographically random password.
		passBytes := make([]byte, 128)
		for read := 0; read != len(passBytes); {
			n, err := rand.Read(passBytes[read:])
			read += n
			if err != nil {
				// Out of random?
				// TODO: Unsure when this can happen.
				sendAuthFail(conn, "Try Later")
				return
			}
		}
		newPass = string(passBytes)

		// Get a cryptographically random salt.
		saltBytes := make([]byte, 32)
		for read := 0; read != len(saltBytes); {
			n, err := rand.Read(saltBytes[read:])
			read += n
			if err != nil {
				// Out of random?
				// TODO: Unsure when this can happen.
				sendAuthFail(conn, "Try Later")
				return
			}
		}

		// Generate their salted, hashed password.
		key, err := scrypt.Key(passBytes, saltBytes, 16384,
			8, 1, 32)
		if err != nil {
			// If you can get here, it's probably
			// because of a bad password.
			sendAuthFail(conn, "Invalid Password")
			return
		}
		hash := string(key)
		salt := string(saltBytes)

		waitingLock.Lock()

		// Create the new user.
		req := makeNewUserRequest(newUser, hash, salt, true)
		go chrequest.Request(req)

		// Stuff details in waiting auth.
		conn.waitingAuth = new(authData)
		conn.waitingAuth.msg = msg
		conn.waitingAuth.requestId = req.RequestId
		waiting[conn.waitingAuth.requestId] = conn

		waitingLock.Unlock()

		return
	}
}

// Can only be called from the handling goroutine for conn.
func handleFollowUsername(conn *userConn, content []byte) {
	var msg cliproto_up.FollowUsername
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.conn.Close()
		return
	}
	// Start transaction.
	store.StartTransaction()
	defer store.EndTransaction()
	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	// Authentication check.
	if conn.session == 0 {
		conn.conn.Close()
		return
	}

	// Lookup this user.
	followId := store.NameLookup("user", "name username", *msg.Username)
	if followId == 0 {
		sendFollowUsernameFail(conn, *msg.Username, "No Such User")
		return
	}

	// Check we're not already following this user.
	// If we are, discard the message.
	for _, existing := range conn.following {
		if existing == followId {
			return
		}
	}

	// Start following this user.
	followUser(conn, followId)
}

// Can only be called from the handling goroutine for conn.
func handleFollowUser(conn *userConn, content []byte) {
	var msg cliproto_up.FollowUser
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.conn.Close()
		return
	}

	// Start transaction.
	store.StartTransaction()
	defer store.EndTransaction()
	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	// Authentication check.
	if conn.session == 0 {
		conn.conn.Close()
		return
	}

	// Check we're not already following this user.
	// If we are, discard the message.
	for _, existing := range conn.following {
		if existing == *msg.UserId {
			return
		}
	}

	// Check this ID is actually a user entity.
	otherUser := store.GetEntity(*msg.UserId)
	if otherUser == nil || otherUser.Value("kind") != "user" {
		sendFollowUserIdFail(conn, *msg.UserId, "No Such User")
		return
	}

	// Start following this user.
	followUser(conn, *msg.UserId)
}

// Can only be called from the handling goroutine for conn.
func handleStopFollowingUser(conn *userConn, content []byte) {
	var msg cliproto_up.StopFollowingUser
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.conn.Close()
		return
	}

	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	// Authentication check.
	if conn.session == 0 {
		conn.conn.Close()
		return
	}

	// If the ID exists in our following list, remove it.
	for i, existing := range conn.following {
		if existing == *msg.UserId {
			conn.following = append(conn.following[:i],
				conn.following[i+1:]...)
		}
	}

	// Send "stopped following" message.
	sendStoppedFollowing(conn, *msg.UserId, "By Request")
}

// Can only be called from the handling goroutine for conn.
func handleSend(conn *userConn, content []byte) {
	var msg cliproto_up.Send
	if err := proto.Unmarshal(content, &msg); err != nil {
		conn.conn.Close()
		return
	}

	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	// Authentication check.
	if conn.session == 0 {
		conn.conn.Close()
		return
	}

	userMsg := new(relay.UserMessage)
	userMsg.Sender = conn.session
	userMsg.Recipient = *msg.Recipient
	userMsg.Tag = *msg.Tag
	userMsg.Content = *msg.Content

	// Deliver this message.
	deliver(userMsg)
}

// Can only be called from the handling goroutine for conn,
// inside a transaction.
func followUser(conn *userConn, followId uint64) {
	conn.following = append(conn.following, followId)

	user := store.GetEntity(followId)

	// Send the user's username.
	var dataMsg cliproto_down.UserData
	dataMsg.UserId = new(uint64)
	dataMsg.Key = new(string)
	dataMsg.Value = new(string)
	dataMsg.FirstUnapplied = new(uint64)
	*dataMsg.UserId = followId
	*dataMsg.Key = "name username"
	*dataMsg.Value = user.Value(*dataMsg.Key)
	*dataMsg.FirstUnapplied = store.InstructionFirstUnapplied()
	conn.conn.SendProto(6, &dataMsg)

	// Send information on all the user's sessions.
	user.Attached(func(key string) {
		*dataMsg.Key = key
		*dataMsg.Value = "true"
		conn.conn.SendProto(6, &dataMsg)
	})

	// Send a done message to indicate that we are finished.
	var doneMsg cliproto_down.UserDataDone
	doneMsg.UserId = dataMsg.UserId
	conn.conn.SendProto(7, &doneMsg)
}

func sendAuthFail(conn *userConn, reason string) {
	var msg cliproto_down.AuthenticationFailed
	msg.Reason = new(string)
	*msg.Reason = reason

	conn.conn.SendProto(2, &msg)
}

func sendAuthSuccess(conn *userConn, password string) {
	var msg cliproto_down.AuthenticationSuccess
	msg.SessionId = new(uint64)
	msg.Password = new(string)
	*msg.SessionId = conn.session
	*msg.Password = password

	conn.conn.SendProto(3, &msg)
}

// Must be called inside transaction.
func sendFollowUsernameFail(conn *userConn, username, reason string) {
	var msg cliproto_down.FollowUsernameFailed
	msg.Username = new(string)
	msg.Reason = new(string)
	msg.FirstUnapplied = new(uint64)
	*msg.Username = username
	*msg.Reason = reason
	*msg.FirstUnapplied = store.InstructionFirstUnapplied()

	conn.conn.SendProto(4, &msg)
}

// Must be called inside transaction.
func sendFollowUserIdFail(conn *userConn, userId uint64, reason string) {
	var msg cliproto_down.FollowUserIdFailed
	msg.UserId = new(uint64)
	msg.Reason = new(string)
	msg.FirstUnapplied = new(uint64)
	*msg.UserId = userId
	*msg.Reason = reason
	*msg.FirstUnapplied = store.InstructionFirstUnapplied()

	conn.conn.SendProto(5, &msg)
}

// Must be called inside transaction.
func sendStoppedFollowing(conn *userConn, userId uint64, reason string) {
	var msg cliproto_down.StoppedFollowing
	msg.UserId = new(uint64)
	msg.Reason = new(string)
	*msg.UserId = userId
	*msg.Reason = reason

	conn.conn.SendProto(9, &msg)
}

// Create change request creating new session entity,
// attached to user entity, this node ID attached to it.
// And the session then set as transient.
func makeNewUserRequest(username, pass, salt string,
	transient bool) *store.ChangeRequest {
	count := 10
	if transient {
		// Add an extra change for setting the user as transient.
		count++
	}
	chset := make([]store.Change, count)

	// Create new entity.
	// Entity ID 1 now refers to this within the changeset.
	chset[0].TargetEntity = 1
	chset[0].Key = "id"
	chset[0].Value = strconv.FormatUint(chset[0].TargetEntity, 10)

	// Make new entity a user entity.
	chset[1].TargetEntity = 1
	chset[1].Key = "kind"
	chset[1].Value = "user"

	// Set username.
	chset[2].TargetEntity = 1
	chset[2].Key = "name username"
	chset[2].Value = username

	// Set password.
	chset[3].TargetEntity = 1
	chset[3].Key = "auth password"
	chset[3].Value = pass

	// Set salt.
	chset[4].TargetEntity = 1
	chset[4].Key = "auth salt"
	chset[4].Value = salt

	// Create new entity.
	// Entity ID 2 now refers to this within the changeset.
	chset[5].TargetEntity = 2
	chset[5].Key = "id"
	chset[5].Value = strconv.FormatUint(chset[0].TargetEntity, 10)

	// Make new entity a session entity.
	chset[6].TargetEntity = 2
	chset[6].Key = "kind"
	chset[6].Value = "session"

	// Attach session entity to user.
	chset[7].TargetEntity = 1
	chset[7].Key = "attach 2"
	chset[7].Value = "true"

	// Attach node ID to session entity.
	idStr := strconv.FormatUint(uint64(config.Id()), 10)
	chset[8].TargetEntity = 2
	chset[8].Key = "attach " + idStr
	chset[8].Value = "true"

	// Set session entity as transient.
	chset[9].TargetEntity = 2
	chset[9].Key = "transient"
	chset[9].Value = "true"

	// Set user entity as transient, if the user is to be so.
	if transient {
		chset[10].TargetEntity = 1
		chset[10].Key = "transient"
		chset[10].Value = "true"
	}

	return makeRequest(chset)
}

// Create change request creating new session entity,
// attached to user entity, this node ID attached to it.
// And the session then set as transient.
func makeNewSessionRequest(userId uint64) *store.ChangeRequest {
	chset := make([]store.Change, 5)

	// Create new entity.
	// Entity ID 1 now refers to this within the changeset.
	chset[0].TargetEntity = 1
	chset[0].Key = "id"
	chset[0].Value = strconv.FormatUint(chset[0].TargetEntity, 10)

	// Make new entity a session entity.
	chset[1].TargetEntity = 1
	chset[1].Key = "kind"
	chset[1].Value = "session"

	// Attach session entity to user.
	chset[2].TargetEntity = userId
	chset[2].Key = "attach 1"
	chset[2].Value = "true"

	// Attach node ID to session entity.
	idStr := strconv.FormatUint(uint64(config.Id()), 10)
	chset[3].TargetEntity = 1
	chset[3].Key = "attach " + idStr
	chset[3].Value = "true"

	// Set session entity as transient.
	chset[4].TargetEntity = 1
	chset[4].Key = "transient"
	chset[4].Value = "true"

	return makeRequest(chset)
}

// Must be called from inside a transaction.
func makeRequest(changes []store.Change) *store.ChangeRequest {
	req := new(store.ChangeRequest)
	req.RequestEntity = uint64(config.Id())
	req.RequestNode = config.Id()
	req.RequestId = store.AllocateRequestId()
	req.Changeset = changes

	return req
}

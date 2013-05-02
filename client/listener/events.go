package listener

import (
	"strconv"
	"strings"
)

import (
	"github.com/jbeshir/unanimity/config"
	"github.com/jbeshir/unanimity/shared/store"
)

func init() {
	store.AddDegradedCallback(handleDegraded)
	store.AddDeletingCallback(handleDeleting)
	store.AddAppliedCallback(handleApplied)
}

func handleDegraded() {

	// If we become degraded, drop all clients.
	if store.Degraded() {
		connectionsLock.Lock()

		for conn, _ := range connections {
			conn.conn.Close()
		}
		connections = make(map[*userConn]bool)

		sessionsLock.Lock()
		sessions = make(map[uint64]*userConn)
		sessionsLock.Unlock()

		waitingLock.Lock()
		waiting = make(map[uint64]*userConn)
		waitingLock.Unlock()

		connectionsLock.Unlock()

	} else {
		sessionsLock.Lock()
		defer sessionsLock.Unlock()

		// If we've become undegraded...
		// Check for attached sessions without connections.
		checkOrphanAttaches()

		// Check for nameless users.
		checkNameless()
	}
}

func handleApplied(slot uint64, idemChanges []store.Change) {

	relativeSlot := int(slot - store.InstructionStart())
	slots := store.InstructionSlots()
	requestId := slots[relativeSlot][0].ChangeRequest().RequestId

	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	waitingLock.Lock()
	defer waitingLock.Unlock()

	// If there is a connection which has this as its waiting request,
	// have it complete authentication.
	waitingConn := waiting[requestId]
	if waitingConn != nil {
		handleAuthComplete(waitingConn, idemChanges)
	}

	ourAttachStr := "attach " + strconv.FormatUint(uint64(config.Id()), 10)
	for i := range idemChanges {

		key := idemChanges[i].Key
		value := idemChanges[i].Value

		if strings.HasPrefix(key, "attach ") {

			// If we were just detached from a session,
			// drop any connections associated with it.
			if key == ourAttachStr && value == "" {

				sessionId := idemChanges[i].TargetEntity
				if conn := sessions[sessionId]; conn != nil {
					conn.conn.Close()
				}
			}

			// Handle a session attaching/detaching from a user.
			entityId := idemChanges[i].TargetEntity
			entity := store.GetEntity(entityId)
			if entity != nil && entity.Value("kind") == "user" {

				// Tell following connections when a session
				// attaches or detaches from a followed user.
				// TODO: Expensive loop,
				// should maintain index to avoid this.
				for conn, _ := range connections {
					following := false
					for _, fw := range conn.following {
						if fw == entityId {
							following = true
							break
						}
					}

					if !following {
						continue
					}

					sendUserChange(conn, entityId,
						key, value)
				}
			}
		}
	}

	// If we're attached to any sessions we lack connections for...
	// - TODO: Start timer to remove ourselves from them.

	// If we've created a nameless user, start a timer to delete it.
	for i := range idemChanges {
		if idemChanges[i].Key != "kind" ||
			idemChanges[i].Value != "user" {

			continue
		}

		newUserId := idemChanges[i].TargetEntity
		newUser := store.GetEntity(newUserId)
		if newUser != nil && newUser.Value("name username") == "" {
			startNamelessTimeout(newUserId)
		}
	}
}

func handleDeleting(entityId uint64) {

	entity := store.GetEntity(entityId)
	kind := entity.Value("kind")

	switch kind {

	// If it's a user deletion, find any sessions attached to the user,
	// and drop any connections attached to those.
	// Cancel any follows of that user.
	// Additionally, remove any corresponding nameless user delete timeout.
	case "user":
		connectionsLock.Lock()
		defer connectionsLock.Unlock()

		// TODO: Expensive loop, should maintain index to avoid this.
		for conn, _ := range connections {
			for i, followed := range conn.following {
				if followed != entityId {
					continue
				}

				fwing := conn.following
				fwing = append(fwing[:i], fwing[i+1:]...)
				conn.following = fwing

				sendStoppedFollowing(conn, entityId,
					"User Deleted")
			}
		}

		sessionIds := entity.AllAttached()
		if len(sessionIds) > 0 {
			sessionsLock.Lock()
			defer sessionsLock.Unlock()

			for _, sessionId := range sessionIds {
				if conn := sessions[sessionId]; conn != nil {
					conn.conn.Close()
				}
			}
		}

		if namelessRemoveTimeouts[entityId] != nil {
			namelessRemoveTimeouts[entityId].Stop()
			delete(namelessRemoveTimeouts, entityId)
		}

	// If it's a session, and we have a connection associated to it,
	// close that connection and remove it from our map.
	case "session":
		sessionsLock.Lock()
		defer sessionsLock.Unlock()

		if conn := sessions[entityId]; conn != nil {
			conn.conn.Close()
			delete(sessions, entityId)
		}
	}
}

// Must be called in a transaction, holding session and waiting locks.
func handleAuthComplete(conn *userConn, idemChanges []store.Change) {

	// We're no longer waiting for authentication to complete.
	delete(waiting, conn.waitingAuth.requestId)
	waitingAuth := conn.waitingAuth
	conn.waitingAuth = nil

	// Find the created session entity.
	newSessionId := uint64(0)
	for i := range idemChanges {
		if idemChanges[i].Key == "kind" &&
			idemChanges[i].Value == "session" {

			newSessionId = idemChanges[i].TargetEntity
			break
		}
	}

	// Check session exists.
	session := store.GetEntity(newSessionId)
	if session == nil ||
		session.Value("kind") != "session" {
		sendAuthFail(conn, "Session Deleted")
		return
	}

	// Check it is attached to a user.
	attachedTo := store.AllAttachedTo(newSessionId)
	if len(attachedTo) == 0 {
		sendAuthFail(conn, "User Deleted")
		return
	}

	// Check that user has a username.
	user := store.GetEntity(attachedTo[0])
	if user.Value("name username") == "" {
		sendAuthFail(conn, "Username Already In Use")

		// TODO: Should request deletion of user, leaning on timeout.
		return
	}

	// This connection is successfully authenticated with that session.
	conn.session = newSessionId

	// If there is an existing connection associated with that session,
	// drop it.
	if existingConn := sessions[newSessionId]; existingConn != nil {
		existingConn.conn.Close()
	}

	sessions[conn.session] = conn
	sendAuthSuccess(conn, waitingAuth.password)
}

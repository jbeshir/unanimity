package store

import (
	"strconv"
	"strings"
)

type Store struct {
	values map[string]string
}

// Map of entity IDs to entity stores.
var entityMap map[uint64]*Store

// Global key-value store, instance of the Store struct.
var Global Store

// Creates the Global store and entity map on startup.
func init() {
	entityMap = make(map[uint64]*Store)
	Global = newStore()
}

// Looks up the given key in the given store.
// If there is no such key, returns an empty string.
func (s *Store) Value(key string) string {
	v, _ := s.values[key]
	return v
}

// Enumerates the attach keys on the given entity.
func (s *Store) Attached(cb func(key string)) {

	for key := range s.values {
		if !strings.HasPrefix(key, "attach ") {
			continue
		}

		cb(key)
	}
}

// Returns all attached IDs as a slice.
func (s *Store) AllAttached() []uint64 {

	ids := make([]uint64, 0)

	for key := range s.values {
		if !strings.HasPrefix(key, "attach ") {
			continue
		}

		// If there's a problem here, the shared state is corrupt.
		// We don't handle that.
		idStr := key[7:]
		id, _ := strconv.ParseUint(idStr, 10, 64)
		ids = append(ids, id)
	}

	return ids
}

// Returns the entity store for the entity with the given ID,
// or nil if there is no such entity.
func GetEntity(id uint64) *Store {
	s, _ := entityMap[id]
	return s
}

// Returns the IDs of all entities the specified entity is attached to.
// Returns an empty slice if the entity doesn't exist.
func AllAttachedTo(id uint64) []uint64 {

	attachedTo := make([]uint64, 0)

	attachKey := "attach " + strconv.FormatUint(id, 10)

	// TODO: Need indexes to make this not need iterating the entire store.
	for otherEntity, store := range entityMap {
		if _, exists := store.values[attachKey]; exists {
			attachedTo = append(attachedTo, otherEntity)
		}
	}

	return attachedTo
}

// Calls the given callbacks for all global and all entity data.
// Changes are allowed to occur while this is ongoing,
// but each entity will be serialised atomically.
// If a callback returns false, stops immediately.
// Must NOT be called while within a transaction.
//
// Will always start and end a transaction such that after ending transaction,
// no further data is sent, so as to ensure that any changes which occurred
// during the burst have completed before returning.
// This ensures that an applied callback will have been called for all changes
// made while bursting, before this function returns.
func Burst(globalCb func(key, value string) (stop bool),
	entityCb func(entity uint64, key, value string) (stop bool)) {

	StartTransaction()
	defer EndTransaction()

	for key, value := range Global.values {
		if !globalCb(key, value) {
			// Burst aborted, return.
			return
		}
	}
	for entity, store := range entityMap {

		entityCb(entity, "id", strconv.FormatUint(entity, 10))

		for key, value := range store.values {

			if key == "id" {
				continue
			}
			if !entityCb(entity, key, value) {
				// Burst aborted, return.
				return
			}
		}

		// Potentially allow another goroutine to act.
		EndTransaction()
		StartTransaction()
	}
}

// Sets the given key and value in the global store.
// May only be called while degraded.
func BurstGlobal(key, value string) {
	if !degraded {
		panic("tried to set global state while not degraded")
	}

	Global.values[key] = value
}

// Sets the given key and value on the given entity.
// May only be called while degraded.
func BurstEntity(entity uint64, key, value string) {
	if !degraded {
		panic("tried to set global state while not degraded")
	}

	// Handle the ID key, which is interpreted as an instruction to
	// create or delete an entity.
	if key == "id" {
		if value != "" {
			store := newStore()
			entityMap[entity] = &store
			entityMap[entity].values[key] =
				strconv.FormatUint(entity, 10)
		} else {
			delete(entityMap, entity)
		}

		return
	}

	entityMap[entity].values[key] = value
}

// Creates a new Store.
func newStore() Store {
	return Store{values: make(map[string]string)}
}

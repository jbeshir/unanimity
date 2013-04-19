package store

type Store struct {
	values map[string]string
}

// Map of entity IDs to entity stores.
var entityMap map[uint64]*Store

// Global key-value store, instance of the Store struct.
var Global Store

// Looks up the given key in the given store.
// If there is no such key, returns an empty string.
func (s *Store) Value(key string) string {
	v, _ := s.values[key]
	return v
}

// Returns the entity store for the entity with the given ID,
// or nil if there is no such entity.
func GetEntity(id uint64) *Store {
	s, _ := entityMap[id]
	return s
}

// Creates a new Store.
func newStore() Store {
	return Store{values: make(map[string]string)}
}

// Creates the Global store and entity map on startup.
func init() {
	entityMap = make(map[uint64]*Store)
	Global = newStore()
}

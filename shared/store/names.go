package store

// Returns the entity ID of the entity with the specified kind,
// such that the given name-prefixed key equals the given value.
// Returns 0 if no such entity exists.
// May only be called in a transaction.
func NameLookup(entityKind, nameKey, nameValue string) uint64 {

	// While this works for non-"name"-prefixed keys right now,
	// that will likely cease to be the case when indexes are implemented.

	// TODO: Keep some kind of indexes so we don't need to
	// do this by iterating through all entities.
	for id, store := range entityMap {
		if store.values["kind"] != entityKind {
			continue
		}
		if store.values[nameKey] == nameValue {
			return id
		}
	}

	return 0
}

// Returns the IDs of all nameless entities with the specified kind.
// May only be called in a transaction.
func Nameless(entityKind, nameKey string) []uint64 {

	// While this works for non-"name"-prefixed keys right now,
	// that will likely cease to be the case when indexes are implemented.

	ids := make([]uint64, 0)

	// TODO: Keep some kind of indexes so we don't need to
	// do this by iterating through all entities.
	for id, store := range entityMap {
		if store.values["kind"] != entityKind {
			continue
		}
		if store.values[nameKey] == "" {
			ids = append(ids, id)
		}
	}

	return ids
}

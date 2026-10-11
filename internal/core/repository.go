package core

// Repository is the persistence boundary for one authorized workspace.
// Reads must return errors instead of silently substituting an empty dataset.
// A future PostgreSQL implementation must scope every operation to a verified
// organization and use a transaction for each update.
//
// Update calls the function with a private mutable copy and either commits
// all changes or none. Implementations must not retry a callback that has
// external side effects (the reconciliation callbacks only mutate data).
type Repository interface {
	Read() (Database, error)
	Update(func(*Database) error) error
}

// The existing JSON-backed single-workspace pilot implements Repository.
var _ Repository = (*Store)(nil)

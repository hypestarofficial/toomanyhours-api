package dbrepo

import "toomanyhours-api/internal/repository"

// PostgresDBRepo is only ever used as a concrete type (application.DB is
// *PostgresDBRepo, not the interface), so nothing would otherwise notice if a
// method were added to DatabaseRepo and not implemented here — or implemented
// with a drifting signature. This assertion turns that into a compile error.
//
// It lives in a _test file so it costs nothing at build time.
var _ repository.DatabaseRepo = (*PostgresDBRepo)(nil)

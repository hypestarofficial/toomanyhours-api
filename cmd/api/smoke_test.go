package main

import (
	"net/http"
	"testing"
)

func TestHarnessServesTheRealRouter(t *testing.T) {
	app, _ := newTestApp(t)

	w := do(t, app, http.MethodGet, "/", nil)

	mustStatus(t, w, http.StatusOK)
}

// The next two are a pair and depend on running in this order. Go runs tests
// in the order they are declared, so A writes and B checks the write is gone.
// One self-contained test could not tell rollback from a fresh database.

func TestHarnessRollbackA_WritesCanary(t *testing.T) {
	_, tx := newTestApp(t)

	createUser(t, tx, "rollbackcanary", "public")
}

func TestHarnessRollbackB_SeesNoCanary(t *testing.T) {
	_, tx := newTestApp(t)

	var count int64
	if err := tx.Raw(`SELECT COUNT(*) FROM users WHERE username = ?`, "rollbackcanary").Scan(&count).Error; err != nil {
		t.Fatalf("count canary: %v", err)
	}
	if count != 0 {
		t.Fatalf("found %d canary rows from the previous test — the transaction is not being rolled back", count)
	}
}

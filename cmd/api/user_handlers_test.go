package main

import (
	"net/http"
	"strings"
	"testing"

	"toomanyhours-api/internal/models"
)

// reloadUser reads the row back, because PatchMe builds its response from the
// in-memory struct: asserting only on the response passes even when nothing
// was written. The update allowlist in UpdateUser is an explicit Select, so a
// field missing from it is dropped silently — this is what catches that.
func reloadUser(t *testing.T, app *application, id int) *models.User {
	t.Helper()
	var u models.User
	if err := app.DB.GormDB.First(&u, id).Error; err != nil {
		t.Fatalf("reload user %d: %v", id, err)
	}
	return &u
}

func TestPatchMeSetsTheBio(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	w := do(t, app, http.MethodPatch, "/me", map[string]any{"bio": "  Plays too much Bethesda.  "}, withAuth(token))
	mustStatus(t, w, http.StatusOK)

	var body struct {
		Bio *string `json:"bio"`
	}
	decodeJSON(t, w, &body)

	// Trimmed by validate.Bio on the way in.
	if body.Bio == nil || *body.Bio != "Plays too much Bethesda." {
		t.Errorf("bio = %v, want the trimmed bio", body.Bio)
	}

	stored := reloadUser(t, app, user.ID)
	if stored.Bio == nil || *stored.Bio != "Plays too much Bethesda." {
		t.Errorf("stored bio = %v, want it persisted", stored.Bio)
	}
}

// The request that carries only a bio is also the request that clears one, and
// PatchMe's "nothing to update" guard rejects a patch whose every known field
// is nil. Forgetting to teach it about bio makes exactly this call fail.
//
// It is also the case GORM can quietly break: a nil *string is a zero value,
// and some update forms skip those — which would return 200 while changing
// nothing.
func TestPatchMeClearsTheBio(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	if err := tx.Model(user).Update("bio", "something").Error; err != nil {
		t.Fatalf("set bio: %v", err)
	}
	token := accessToken(t, app, user)

	w := do(t, app, http.MethodPatch, "/me", map[string]any{"bio": ""}, withAuth(token))
	mustStatus(t, w, http.StatusOK)

	var body struct {
		Bio *string `json:"bio"`
	}
	decodeJSON(t, w, &body)

	if body.Bio != nil {
		t.Errorf("bio = %q, want null after clearing", *body.Bio)
	}

	stored := reloadUser(t, app, user.ID)
	if stored.Bio != nil {
		t.Errorf("stored bio = %q, want null after clearing", *stored.Bio)
	}
}

func TestPatchMeRejectsAnOverLongBio(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	w := do(t, app, http.MethodPatch, "/me", map[string]any{"bio": strings.Repeat("a", 501)}, withAuth(token))
	mustStatus(t, w, http.StatusBadRequest)
}

// An empty patch is still an error; adding bio must not turn "every field
// optional" into "every field absent is fine".
func TestPatchMeWithNothingIs400(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	w := do(t, app, http.MethodPatch, "/me", map[string]any{}, withAuth(token))
	mustStatus(t, w, http.StatusBadRequest)
}

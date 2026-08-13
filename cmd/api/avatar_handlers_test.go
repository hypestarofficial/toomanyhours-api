package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"testing"
)

// pngUpload builds a real multipart body with a real PNG in it.
func pngUpload(t *testing.T, w, h int) (*bytes.Buffer, string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 200, A: 255})
		}
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("avatar", "photo.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(raw.Bytes()); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return body, mw.FormDataContentType()
}

func TestPutAvatarStoresItAndMeReturnsIt(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	body, ctype := pngUpload(t, 400, 300)
	w := doRaw(t, app, http.MethodPut, "/me/avatar", body, ctype, withAuth(token))
	mustStatus(t, w, http.StatusOK)

	stored, err := app.DB.GetUserAvatar(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("avatar was not stored: %v", err)
	}
	if len(stored.Bytes) == 0 || stored.Hash == "" {
		t.Errorf("stored = %d bytes, hash %q, want both set", len(stored.Bytes), stored.Hash)
	}

	// GET /me carries it inline, because an <img> cannot send a bearer token.
	me := do(t, app, http.MethodGet, "/me", nil, withAuth(token))
	mustStatus(t, me, http.StatusOK)

	var meBody struct {
		Avatar *string `json:"avatar"`
	}
	decodeJSON(t, me, &meBody)

	if meBody.Avatar == nil || !bytes.HasPrefix([]byte(*meBody.Avatar), []byte("data:image/jpeg;base64,")) {
		t.Errorf("avatar = %v, want a jpeg data URI", meBody.Avatar)
	}
}

// A replacement must change the hash, or the cache-busting URL never changes
// and the old photo stays on screen forever.
func TestPutAvatarTwiceChangesTheHash(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	first, ctype := pngUpload(t, 400, 300)
	mustStatus(t, doRaw(t, app, http.MethodPut, "/me/avatar", first, ctype, withAuth(token)), http.StatusOK)
	before, err := app.DB.GetUserAvatar(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("first upload missing: %v", err)
	}

	second, ctype2 := pngUpload(t, 900, 900)
	mustStatus(t, doRaw(t, app, http.MethodPut, "/me/avatar", second, ctype2, withAuth(token)), http.StatusOK)
	after, err := app.DB.GetUserAvatar(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("second upload missing: %v", err)
	}

	if before.Hash == after.Hash {
		t.Error("the hash did not change, so a cache-busted URL would not either")
	}
}

func TestDeleteAvatarRemovesIt(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	body, ctype := pngUpload(t, 300, 300)
	mustStatus(t, doRaw(t, app, http.MethodPut, "/me/avatar", body, ctype, withAuth(token)), http.StatusOK)
	mustStatus(t, do(t, app, http.MethodDelete, "/me/avatar", nil, withAuth(token)), http.StatusOK)

	me := do(t, app, http.MethodGet, "/me", nil, withAuth(token))
	var meBody struct {
		Avatar *string `json:"avatar"`
	}
	decodeJSON(t, me, &meBody)

	if meBody.Avatar != nil {
		t.Error("avatar still present after delete")
	}
}

func TestPutAvatarRejectsANonImage(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("avatar", "evil.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("not an image, whatever the filename claims")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	w := doRaw(t, app, http.MethodPut, "/me/avatar", body, mw.FormDataContentType(), withAuth(token))
	mustStatus(t, w, http.StatusBadRequest)
}

func TestGetProfileAvatarServesTheBytes(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	body, ctype := pngUpload(t, 400, 400)
	mustStatus(t, doRaw(t, app, http.MethodPut, "/me/avatar", body, ctype, withAuth(token)), http.StatusOK)
	stored, err := app.DB.GetUserAvatar(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("upload missing: %v", err)
	}

	// No token: this is the route a visitor uses.
	w := do(t, app, http.MethodGet, "/profiles/hype/avatar?v="+stored.Hash, nil)
	mustStatus(t, w, http.StatusOK)

	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	if !bytes.Equal(w.Body.Bytes(), stored.Bytes) {
		t.Error("served bytes differ from the stored ones")
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want the immutable form for a matching hash", cc)
	}
}

// A stale or absent v must not be cached forever, or a hand-typed URL pins an
// old photo permanently.
func TestGetProfileAvatarWithoutMatchingHashIsNotImmutable(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	body, ctype := pngUpload(t, 400, 400)
	mustStatus(t, doRaw(t, app, http.MethodPut, "/me/avatar", body, ctype, withAuth(token)), http.StatusOK)

	for _, target := range []string{"/profiles/hype/avatar", "/profiles/hype/avatar?v=deadbeef"} {
		w := do(t, app, http.MethodGet, target, nil)
		mustStatus(t, w, http.StatusOK)
		if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, want no-cache", target, cc)
		}
	}
}

func TestGetProfileAvatarPrivateIs403(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "shy", "private")
	token := accessToken(t, app, user)

	body, ctype := pngUpload(t, 300, 300)
	mustStatus(t, doRaw(t, app, http.MethodPut, "/me/avatar", body, ctype, withAuth(token)), http.StatusOK)

	mustStatus(t, do(t, app, http.MethodGet, "/profiles/shy/avatar", nil), http.StatusForbidden)
}

func TestGetProfileAvatarWithoutOneIs404(t *testing.T) {
	app, tx := newTestApp(t)
	createUser(t, tx, "bare", "public")

	mustStatus(t, do(t, app, http.MethodGet, "/profiles/bare/avatar", nil), http.StatusNotFound)
	mustStatus(t, do(t, app, http.MethodGet, "/profiles/nobody/avatar", nil), http.StatusNotFound)
}

// The literal-list trap, third occurrence in this codebase. PatchMe builds its
// own APIUser, so an avatar can be present on GET /me and absent from the
// response to a username change — which blanks the navbar until a reload.
func TestPatchMeKeepsTheAvatar(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	token := accessToken(t, app, user)

	body, ctype := pngUpload(t, 300, 300)
	mustStatus(t, doRaw(t, app, http.MethodPut, "/me/avatar", body, ctype, withAuth(token)), http.StatusOK)

	w := do(t, app, http.MethodPatch, "/me", map[string]any{"username": "hype2"}, withAuth(token))
	mustStatus(t, w, http.StatusOK)

	var patched struct {
		Avatar *string `json:"avatar"`
	}
	decodeJSON(t, w, &patched)

	if patched.Avatar == nil {
		t.Error("PatchMe's response dropped the avatar; the navbar would blank until a reload")
	}
}

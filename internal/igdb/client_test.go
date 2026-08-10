package igdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeIGDB stands in for both Twitch and IGDB. Pointing the client at it is
// what the configurable base URLs exist for: the whole package is exercised
// without a network or a credential.
type fakeIGDB struct {
	server *httptest.Server

	tokenCalls  atomic.Int32
	searchCalls atomic.Int32

	mu           sync.Mutex
	token        string
	expiresIn    int
	rejectToken  string // when a search presents this token, answer 401
	searchBody   string
	searchStatus int
	lastRequest  string // the Apicalypse body most recently received
}

func newFakeIGDB(t *testing.T) *fakeIGDB {
	t.Helper()

	f := &fakeIGDB{token: "token-1", expiresIn: 5184000, searchBody: "[]", searchStatus: http.StatusOK}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenCalls.Add(1)
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":%q,"expires_in":%d,"token_type":"bearer"}`, f.token, f.expiresIn)
	})
	mux.HandleFunc("/games", func(w http.ResponseWriter, r *http.Request) {
		f.searchCalls.Add(1)
		body, _ := io.ReadAll(r.Body)

		f.mu.Lock()
		defer f.mu.Unlock()
		f.lastRequest = string(body)

		if f.rejectToken != "" && r.Header.Get("Authorization") == "Bearer "+f.rejectToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.searchStatus)
		fmt.Fprint(w, f.searchBody)
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeIGDB) lastBody() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastRequest
}

func (f *fakeIGDB) client(now func() time.Time) *Client {
	c := New(Config{
		ClientID:     "id",
		ClientSecret: "secret",
		TokenURL:     f.server.URL + "/oauth2/token",
		APIURL:       f.server.URL,
		Now:          now,
	})

	// Pacing is proven in throttle_test.go with a frozen clock. Sleeping for
	// real here would add nearly two seconds to the concurrency test and prove
	// nothing these tests are about.
	c.throttle.sleep = func(context.Context, time.Duration) error { return nil }
	return c
}

func TestTokenIsFetchedOnceAndCached(t *testing.T) {
	f := newFakeIGDB(t)
	c := f.client(time.Now)

	for i := 0; i < 3; i++ {
		if _, err := c.Search(context.Background(), "witcher", 10); err != nil {
			t.Fatalf("search %d: %v", i, err)
		}
	}

	if got := f.tokenCalls.Load(); got != 1 {
		t.Fatalf("token fetched %d times, want 1", got)
	}
}

func TestTokenIsRefreshedNearExpiry(t *testing.T) {
	f := newFakeIGDB(t)
	f.expiresIn = 120 // two minutes

	clock := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	c := f.client(func() time.Time { return clock })

	if _, err := c.Search(context.Background(), "a", 10); err != nil {
		t.Fatal(err)
	}
	// Inside the refresh margin: the cached token would still be technically
	// valid, but not for long enough to trust with a request in flight.
	clock = clock.Add(90 * time.Second)
	if _, err := c.Search(context.Background(), "b", 10); err != nil {
		t.Fatal(err)
	}

	if got := f.tokenCalls.Load(); got != 2 {
		t.Fatalf("token fetched %d times, want 2", got)
	}
}

func TestUnauthorizedRefreshesOnceAndRetries(t *testing.T) {
	f := newFakeIGDB(t)
	c := f.client(time.Now)

	// Prime the cache, then revoke that token server-side — which is exactly
	// what a token being revoked before its expiry looks like.
	if _, err := c.Search(context.Background(), "warm", 10); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.rejectToken = "token-1"
	f.token = "token-2"
	f.mu.Unlock()

	if _, err := c.Search(context.Background(), "again", 10); err != nil {
		t.Fatalf("search after revocation: %v", err)
	}
	if got := f.tokenCalls.Load(); got != 2 {
		t.Fatalf("token fetched %d times, want 2", got)
	}
}

func TestPersistentUnauthorizedFailsRatherThanLooping(t *testing.T) {
	f := newFakeIGDB(t)
	c := f.client(time.Now)

	// Every token is rejected, so the retry cannot help. One retry, then an
	// error — anything else is an unbounded refresh loop against Twitch.
	f.mu.Lock()
	f.rejectToken = "token-1"
	f.mu.Unlock()

	_, err := c.Search(context.Background(), "x", 10)
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("error = %v, want ErrUpstream", err)
	}
	if got := f.searchCalls.Load(); got != 2 {
		t.Fatalf("search attempted %d times, want 2", got)
	}
}

func TestConcurrentCallersFetchOneToken(t *testing.T) {
	f := newFakeIGDB(t)
	c := f.client(time.Now)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.Search(context.Background(), "concurrent", 10)
		}()
	}
	wg.Wait()

	if got := f.tokenCalls.Load(); got != 1 {
		t.Fatalf("token fetched %d times, want 1", got)
	}
}

func TestMissingCredentialsAreNotConfigured(t *testing.T) {
	c := New(Config{})

	if _, err := c.Search(context.Background(), "witcher", 10); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}

func TestUpstreamFailureIsReported(t *testing.T) {
	f := newFakeIGDB(t)
	f.searchStatus = http.StatusInternalServerError
	f.searchBody = `{"message":"internal detail that must not leak"}`
	c := f.client(time.Now)

	_, err := c.Search(context.Background(), "witcher", 10)
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("error = %v, want ErrUpstream", err)
	}
	if strings.Contains(err.Error(), "internal detail") {
		t.Fatalf("upstream body leaked into the error: %v", err)
	}
}

// A response with everything present, and one with nothing optional present.
// IGDB really does return games with no cover and no announced date, and a
// parser that assumes otherwise crashes on an ordinary search.
const searchFixture = `[
  {
    "id": 1942,
    "name": "The Witcher 3: Wild Hunt",
    "cover": {"id": 89386, "image_id": "co1wyy"},
    "first_release_date": 1431993600,
    "genres": [{"id": 12, "name": "Role-playing (RPG)"}, {"id": 31, "name": "Adventure"}],
    "themes": [{"id": 17, "name": "Fantasy"}],
    "game_modes": [{"id": 1, "name": "Single player"}]
  },
  {
    "id": 999999,
    "name": "Unannounced Thing"
  }
]`

func TestSearchParsesResults(t *testing.T) {
	f := newFakeIGDB(t)
	f.searchBody = searchFixture
	c := f.client(time.Now)

	games, err := c.Search(context.Background(), "witcher", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 2 {
		t.Fatalf("got %d games, want 2", len(games))
	}

	w := games[0]
	if w.IGDBID != 1942 || w.Title != "The Witcher 3: Wild Hunt" {
		t.Fatalf("first game = %+v", w)
	}
	if w.Image == nil || *w.Image != "https://images.igdb.com/igdb/image/upload/t_cover_big/co1wyy.jpg" {
		t.Fatalf("image = %v", w.Image)
	}
	if w.ReleaseDate == nil || *w.ReleaseDate != "2015-05-19" {
		t.Fatalf("releaseDate = %v", w.ReleaseDate)
	}
	if len(w.Genres) != 2 || w.Genres[0].IGDBID != 12 || w.Genres[0].Name != "Role-playing (RPG)" {
		t.Fatalf("genres = %+v", w.Genres)
	}
	if len(w.Themes) != 1 || w.Themes[0].IGDBID != 17 {
		t.Fatalf("themes = %+v", w.Themes)
	}
	if len(w.GameModes) != 1 || w.GameModes[0].Name != "Single player" {
		t.Fatalf("gameModes = %+v", w.GameModes)
	}
}

func TestSearchHandlesMissingOptionalFields(t *testing.T) {
	f := newFakeIGDB(t)
	f.searchBody = searchFixture
	c := f.client(time.Now)

	games, err := c.Search(context.Background(), "witcher", 10)
	if err != nil {
		t.Fatal(err)
	}

	bare := games[1]
	if bare.Image != nil {
		t.Fatalf("image = %v, want nil", *bare.Image)
	}
	if bare.ReleaseDate != nil {
		t.Fatalf("releaseDate = %v, want nil", *bare.ReleaseDate)
	}
	// Empty, not nil: a nil slice marshals to JSON null, and the same mistake
	// in GetUserGames made an empty list crash the frontend.
	if bare.Genres == nil || bare.Themes == nil || bare.GameModes == nil {
		t.Fatalf("tag slices must be empty, not nil: %+v", bare)
	}
	if len(bare.Genres) != 0 {
		t.Fatalf("genres = %+v, want empty", bare.Genres)
	}
}

// What actually reaches IGDB: the escaped term, the fields the parser expects,
// and the limit. The escaping unit test proves the function; this proves the
// function is wired into the request that gets sent.
func TestSearchSendsAnEscapedApicalypseQuery(t *testing.T) {
	f := newFakeIGDB(t)
	c := f.client(time.Now)

	if _, err := c.Search(context.Background(), `x"; fields *;`, 25); err != nil {
		t.Fatal(err)
	}

	got := f.lastBody()

	// The hostile term stays inside the string literal: with the quote escaped,
	// its semicolons are just characters.
	if !strings.Contains(got, `search "x\"; fields *;";`) {
		t.Fatalf("term was not escaped into the query: %s", got)
	}
	if !strings.Contains(got, "limit 25;") {
		t.Fatalf("limit missing from query: %s", got)
	}
	// Every field the parser reads must be requested, or the parse silently
	// yields zero values rather than failing.
	for _, field := range []string{"cover.image_id", "first_release_date", "genres.id", "themes.id", "game_modes.id"} {
		if !strings.Contains(got, field) {
			t.Fatalf("query does not request %s: %s", field, got)
		}
	}
	// IGDB's `category` means DLC/expansion and would collide with this
	// product's categories. Asking for it is how that confusion starts.
	if strings.Contains(got, "category") {
		t.Fatalf("query requests IGDB's category field: %s", got)
	}
}

func TestSearchParsesKind(t *testing.T) {
	f := newFakeIGDB(t)
	f.searchBody = `[{"id":19457,"name":"Skyrim SE","game_type":{"id":9,"type":"Remaster"}},
	                 {"id":1,"name":"No Type Given"}]`
	c := f.client(time.Now)

	games, err := c.Search(context.Background(), "skyrim", 10)
	if err != nil {
		t.Fatal(err)
	}
	if games[0].Kind != "remaster" {
		t.Fatalf("kind = %q, want remaster", games[0].Kind)
	}
	// A game with no game_type must still import.
	if games[1].Kind != "unknown" {
		t.Fatalf("missing game_type kind = %q, want unknown", games[1].Kind)
	}
}

func TestSearchExcludesNoiseTypes(t *testing.T) {
	f := newFakeIGDB(t)
	c := f.client(time.Now)

	if _, err := c.Search(context.Background(), "witcher", 10); err != nil {
		t.Fatal(err)
	}

	got := f.lastBody()
	for _, id := range []int{5, 12, 13, 14} {
		if !strings.Contains(got, fmt.Sprintf("game_type != %d", id)) {
			t.Fatalf("query does not exclude game_type %d: %s", id, got)
		}
	}
	if !strings.Contains(got, "game_type.type") {
		t.Fatalf("query does not request game_type.type: %s", got)
	}
}

func TestGetByIDsQueriesByIDAndDoesNotExclude(t *testing.T) {
	f := newFakeIGDB(t)
	c := f.client(time.Now)

	if _, err := c.GetByIDs(context.Background(), []int{1942, 9630}); err != nil {
		t.Fatal(err)
	}

	got := f.lastBody()
	if !strings.Contains(got, "where id = (1942,9630);") {
		t.Fatalf("unexpected id clause: %s", got)
	}
	if strings.Contains(got, "search ") {
		t.Fatalf("GetByIDs must not send a search clause: %s", got)
	}
	// The exclusion belongs to Search only. Importing an id someone named
	// explicitly must work even for a Pack / Addon.
	if strings.Contains(got, "game_type !=") {
		t.Fatalf("GetByIDs must not exclude release types: %s", got)
	}
}

func TestGetByIDsWithNoIDsMakesNoRequest(t *testing.T) {
	f := newFakeIGDB(t)
	c := f.client(time.Now)

	games, err := c.GetByIDs(context.Background(), nil)
	if err != nil || len(games) != 0 {
		t.Fatalf("games = %v, err = %v", games, err)
	}
	if got := f.searchCalls.Load(); got != 0 {
		t.Fatalf("made %d requests for an empty id list, want 0", got)
	}
}

// The parser can only fill a field the query asked for. Search and GetByIDs
// share gameFields precisely so this cannot be true of one and not the other.
func TestQueryAsksForParentGame(t *testing.T) {
	if !strings.Contains(gameFields, "parent_game") {
		t.Errorf("gameFields does not request parent_game: %s", gameFields)
	}
}

func TestParsesParentGame(t *testing.T) {
	f := newFakeIGDB(t)
	f.searchBody = `[{"id":140517,"name":"Gears 5: Hivebusters","parent_game":103292},
	                 {"id":1942,"name":"The Witcher 3"}]`
	c := f.client(time.Now)

	games, err := c.Search(context.Background(), "gears", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("got %d games, want 2", len(games))
	}
	if games[0].ParentIGDBID == nil || *games[0].ParentIGDBID != 103292 {
		t.Errorf("parent = %v, want 103292", games[0].ParentIGDBID)
	}
	// A game with no parent must come back nil, not zero: 0 would be an id.
	if games[1].ParentIGDBID != nil {
		t.Errorf("parent = %v, want nil for a game with no parent_game", games[1].ParentIGDBID)
	}
}

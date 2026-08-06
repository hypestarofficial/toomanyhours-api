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

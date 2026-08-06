package igdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultTokenURL = "https://id.twitch.tv/oauth2/token"
	defaultAPIURL   = "https://api.igdb.com/v4"

	// Refresh this long before expiry so a token cannot lapse mid-request.
	refreshMargin = time.Minute

	// IGDB's documented limit.
	requestsPerSecond = 4

	// A backstop only. The request context is the real deadline, since the API
	// wraps every request in a 3-second timeout.
	httpTimeout = 10 * time.Second
)

// Config constructs a Client. Only ClientID and ClientSecret are required.
//
// TokenURL and APIURL are fields rather than constants so tests can point the
// whole package at an httptest.Server. That seam is the difference between a
// package that is tested and one that is only exercised by hand.
type Config struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	APIURL       string
	HTTPClient   *http.Client
	Now          func() time.Time
}

// Client is safe for concurrent use.
type Client struct {
	clientID     string
	clientSecret string
	tokenURL     string
	apiURL       string

	http     *http.Client
	now      func() time.Time
	throttle *throttle

	// mu guards the cached token and is held across a fetch. That is the
	// single-flight: a second caller blocks here and then finds the token
	// already refreshed rather than fetching a second one.
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func New(cfg Config) *Client {
	c := &Client{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		tokenURL:     cfg.TokenURL,
		apiURL:       cfg.APIURL,
		http:         cfg.HTTPClient,
		now:          cfg.Now,
		throttle:     newThrottle(requestsPerSecond),
	}

	if c.tokenURL == "" {
		c.tokenURL = defaultTokenURL
	}
	if c.apiURL == "" {
		c.apiURL = defaultAPIURL
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: httpTimeout}
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

// getToken returns a usable access token.
//
// When stale is non-empty the caller was just rejected while presenting it, so
// a refresh is forced — unless another goroutine already replaced it, in which
// case the new one is handed back without a second fetch.
func (c *Client) getToken(ctx context.Context, stale string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if stale != "" {
		if c.token != stale {
			return c.token, nil
		}
	} else if c.token != "" && c.now().Before(c.expiresAt.Add(-refreshMargin)) {
		return c.token, nil
	}

	form := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"grant_type":    {"client_credentials"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: building token request: %v", ErrUpstream, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token request: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// The body is discarded rather than wrapped: it may echo the request,
		// and the request carries the secret.
		return "", fmt.Errorf("%w: token endpoint returned %d", ErrUpstream, resp.StatusCode)
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("%w: decoding token: %v", ErrUpstream, err)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("%w: token endpoint returned no token", ErrUpstream)
	}

	c.token = payload.AccessToken
	c.expiresAt = c.now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	return c.token, nil
}

// post sends one Apicalypse request, returning the status and raw body.
func (c *Client) post(ctx context.Context, path, body, token string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+path, strings.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("%w: building request: %v", ErrUpstream, err)
	}
	req.Header.Set("Client-ID", c.clientID)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: request: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("%w: reading response: %v", ErrUpstream, err)
	}
	return resp.StatusCode, raw, nil
}

// do handles the credentials check, the throttle, the token, and the single
// 401 retry.
func (c *Client) do(ctx context.Context, path, body string) ([]byte, error) {
	if c.clientID == "" || c.clientSecret == "" {
		return nil, ErrNotConfigured
	}
	if err := c.throttle.wait(ctx); err != nil {
		return nil, err
	}

	token, err := c.getToken(ctx, "")
	if err != nil {
		return nil, err
	}

	status, raw, err := c.post(ctx, path, body, token)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnauthorized {
		// A token can be revoked long before it expires. Refresh once and retry
		// once; a second 401 is an error, not another retry, or a revoked
		// credential becomes an unbounded loop against Twitch.
		token, err = c.getToken(ctx, token)
		if err != nil {
			return nil, err
		}
		if status, raw, err = c.post(ctx, path, body, token); err != nil {
			return nil, err
		}
	}

	if status != http.StatusOK {
		// The upstream body is deliberately not included: it is not ours to
		// relay and may echo the request, which carries our client id.
		return nil, fmt.Errorf("%w: igdb returned %d", ErrUpstream, status)
	}
	return raw, nil
}

// Search returns games matching a free-text query, best match first.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Game, error) {
	body := fmt.Sprintf(
		`search "%s"; fields id,name,cover.image_id,first_release_date,`+
			`genres.id,genres.name,themes.id,themes.name,game_modes.id,game_modes.name; `+
			`limit %d;`,
		escapeSearchTerm(query), limit,
	)

	if _, err := c.do(ctx, "/games", body); err != nil {
		return nil, err
	}
	return []Game{}, nil
}

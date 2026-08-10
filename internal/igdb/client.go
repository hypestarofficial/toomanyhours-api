package igdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// coverBaseURL builds a usable image URL from IGDB's image_id. t_cover_big is
// 264x374, which is what the card grid renders.
const coverBaseURL = "https://images.igdb.com/igdb/image/upload/t_cover_big/"

// apiGame mirrors IGDB's JSON. Kept unexported and separate from Game so the
// shape we expose is ours: renaming a field here cannot silently change the
// API contract.
type apiGame struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Cover *struct {
		ImageID string `json:"image_id"`
	} `json:"cover"`
	FirstReleaseDate *int64 `json:"first_release_date"`
	// A reference, not a scalar: the query asks for game_type.type so this
	// comes back as a name rather than an id.
	GameType *struct {
		Type string `json:"type"`
	} `json:"game_type"`
	// A scalar, not a reference expansion: `fields parent_game;` returns the
	// id directly. Asking for parent_game.id would return an object instead.
	ParentGame *int     `json:"parent_game"`
	Summary    *string  `json:"summary"`
	Genres     []apiTag `json:"genres"`
	Themes     []apiTag `json:"themes"`
	GameModes  []apiTag `json:"game_modes"`
}

type apiTag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func tags(in []apiTag) []Tag {
	// Empty rather than nil: a nil slice marshals to JSON null, and consumers
	// map over this.
	out := make([]Tag, 0, len(in))
	for _, t := range in {
		out = append(out, Tag{IGDBID: t.ID, Name: t.Name})
	}
	return out
}

// gameFields is shared by Search and GetByIDs so a field the parser reads can
// never be missing from one of them. game_type is a reference, so it needs
// .type to come back as a name rather than an id; `category` is deprecated and
// returns null for every game.
const gameFields = `fields id,name,cover.image_id,first_release_date,game_type.type,parent_game,summary,` +
	`genres.id,genres.name,themes.id,themes.name,game_modes.id,game_modes.name;`

// excludeNoise drops the release types nobody is trying to find in a search.
// Weapon and skin packs are almost all Pack / Addon, so that one cut does most
// of the work.
//
// DLC and Expansion are excluded because they belong to a game rather than
// standing beside it: they are reached from their parent's card, where the
// whole set is listed at once. A season pass otherwise fills the results, and
// then the list, with one game — Borderlands 4 alone accounts for two rows in
// this database.
//
// The cost is real and was accepted knowingly: an add-on whose parent is not
// in your list cannot be found at all. Gears 5: Hivebusters is a standalone
// campaign, and Blood and Wine is better than most whole games. Adding the
// parent, ticking the add-on and removing the parent again is the way back.
//
// Bundle, Remaster and Expanded Game deliberately stay. They are the release
// someone actually played, not an addition to one — Fallout 3 GOTY, Skyrim SE
// and GTA V Enhanced are all in this database, and two of them have an IGDB
// parent, which is why the rule turns on kind rather than on having a parent.
var excludeNoise = fmt.Sprintf(
	"where game_type != %d & game_type != %d & game_type != %d & game_type != %d & game_type != %d & game_type != %d;",
	typeDLC, typeExpansion, typeMod, typeFork, typePackAddon, typeUpdate,
)

// Search returns games matching a free-text query, best match first.
//
// IGDB's `search` sorts by relevance and cannot be combined with an explicit
// sort, which is why none is requested.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Game, error) {
	body := fmt.Sprintf(`search "%s"; %s %s limit %d;`,
		escapeSearchTerm(query), gameFields, excludeNoise, limit)

	return c.games(ctx, body)
}

// GetByIDs fetches games by IGDB id, for importing something a user has chosen.
//
// No exclusion clause: the filter on Search exists to keep results readable,
// and refusing to import an id someone named explicitly would be a second,
// invisible rule. Search decides what is easy to find, not what may exist.
//
// Ids are integers, so nothing user-supplied is interpolated into the query.
func (c *Client) GetByIDs(ctx context.Context, ids []int) ([]Game, error) {
	if len(ids) == 0 {
		return []Game{}, nil
	}

	list := make([]string, 0, len(ids))
	for _, id := range ids {
		list = append(list, strconv.Itoa(id))
	}

	body := fmt.Sprintf(`where id = (%s); %s limit %d;`,
		strings.Join(list, ","), gameFields, len(ids))

	return c.games(ctx, body)
}

// dlcLimit bounds one add-on listing. Fifty is far more than any real game has
// once packs and mods are excluded, and it stops a pathological entry from
// returning a thousand rows into a modal.
const dlcLimit = 50

// GetDLCs returns the add-ons of one game: everything IGDB files as DLC or
// Expansion with this game as its parent.
//
// Named for DLC because that is the word people use, and it returns expansions
// too. Packs, mods and updates are excluded here for the same reason search
// excludes them — a wall of weapon skins is not what anyone opened this for.
//
// Deliberately not excludeNoise: that clause now excludes DLC and Expansion,
// which is everything this query is for.
//
// The id is an integer, so nothing user-supplied is interpolated into the
// query; the same reasoning GetByIDs documents.
func (c *Client) GetDLCs(ctx context.Context, parentIGDBID int) ([]Game, error) {
	body := fmt.Sprintf(
		`%s where parent_game = %d & (game_type = %d | game_type = %d); sort first_release_date asc; limit %d;`,
		gameFields, parentIGDBID, typeDLC, typeExpansion, dlcLimit,
	)

	return c.games(ctx, body)
}

// games runs one Apicalypse body against /games and parses the result.
//
// Shared so Search and GetByIDs cannot decode differently: a field handled in
// one and not the other would show up as a game that imports with no cover.
func (c *Client) games(ctx context.Context, body string) ([]Game, error) {
	raw, err := c.do(ctx, "/games", body)
	if err != nil {
		return nil, err
	}

	var parsed []apiGame
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("%w: decoding search results: %v", ErrUpstream, err)
	}

	games := make([]Game, 0, len(parsed))
	for _, p := range parsed {
		g := Game{
			IGDBID:    p.ID,
			Title:     p.Name,
			Kind:      "unknown",
			Genres:    tags(p.Genres),
			Themes:    tags(p.Themes),
			GameModes: tags(p.GameModes),
		}

		// A game with no game_type must still import, so the zero value is a
		// storable slug rather than an empty string.
		if p.GameType != nil {
			g.Kind = kindSlug(p.GameType.Type)
		}

		if p.Summary != nil {
			// Copied rather than aliased, for the same reason as the parent
			// below: p is a loop variable.
			summary := *p.Summary
			g.Summary = &summary
		}

		if p.ParentGame != nil {
			// Copied rather than aliased: p is a loop variable, and sharing
			// the pointer across iterations is a classic way to end up with
			// every game claiming the last one's parent.
			parent := *p.ParentGame
			g.ParentIGDBID = &parent
		}

		if p.Cover != nil && p.Cover.ImageID != "" {
			image := coverBaseURL + p.Cover.ImageID + ".jpg"
			g.Image = &image
		}
		if p.FirstReleaseDate != nil {
			// Unix seconds, UTC. Formatting in local time would show the wrong
			// day either side of midnight — the same class of mistake as the
			// naive timestamps in refresh_tokens.
			date := time.Unix(*p.FirstReleaseDate, 0).UTC().Format("2006-01-02")
			g.ReleaseDate = &date
		}

		games = append(games, g)
	}

	return games, nil
}

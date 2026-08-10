// Command backfill re-imports every game in the catalog from IGDB.
//
// It exists because SQL cannot call IGDB. A migration can add a column but
// cannot fill it with anything upstream owns, so a new field — summary today,
// something else tomorrow — arrives empty on every row that already existed.
// This is the tool that fills it, and it is the reason ImportGames upserts
// with a DoUpdates list rather than DO NOTHING.
//
// Run by hand, never automatically: it spends an IGDB request and rewrites
// every row's title, cover, tags, kind, parent and summary from upstream. That
// is the intent, but it should be deliberate.
//
//	go run ./cmd/backfill
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"toomanyhours-api/internal/database"
	"toomanyhours-api/internal/igdb"
	"toomanyhours-api/internal/models"
	"toomanyhours-api/internal/repository/dbrepo"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	dsn := flag.String("dsn", database.DSNFromEnv(), "Postgres connection string")
	clientID := flag.String("igdb-client-id", os.Getenv("IGDB_CLIENT_ID"), "Twitch application client id")
	clientSecret := flag.String("igdb-client-secret", os.Getenv("IGDB_CLIENT_SECRET"), "Twitch application client secret")
	flag.Parse()

	if *clientID == "" || *clientSecret == "" {
		// Fatal here, unlike in the API: an API without credentials still
		// serves lists, but a backfill without them has nothing to do.
		log.Fatal("IGDB credentials are required; set IGDB_CLIENT_ID and IGDB_CLIENT_SECRET")
	}

	gormDB, err := database.Open(*dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	repo := &dbrepo.PostgresDBRepo{GormDB: gormDB}
	client := igdb.New(igdb.Config{ClientID: *clientID, ClientSecret: *clientSecret})

	ctx := context.Background()

	ids, err := repo.AllGameIGDBIDs(ctx)
	if err != nil {
		log.Fatalf("read catalog: %v", err)
	}
	if len(ids) == 0 {
		log.Println("catalog is empty, nothing to do")
		return
	}
	log.Printf("re-importing %d games from IGDB", len(ids))

	// One request: GetByIDs sets `limit len(ids)`, so there is no hidden cap
	// to page around at this size.
	fetched, err := client.GetByIDs(ctx, ids)
	if err != nil {
		log.Fatalf("igdb: %v", err)
	}
	if len(fetched) != len(ids) {
		// Not fatal: a game IGDB has forgotten should not stop the rest being
		// refreshed. Worth saying out loud, though.
		log.Printf("warning: asked for %d games, IGDB returned %d", len(ids), len(fetched))
	}

	games := make([]*models.Game, 0, len(fetched))
	for _, g := range fetched {
		games = append(games, models.FromIGDB(g))
	}

	if _, err := repo.ImportGames(ctx, games); err != nil {
		log.Fatalf("import: %v", err)
	}

	log.Printf("re-imported %d games", len(games))
}

package main

import (
	"context"
	"testing"

	"toomanyhours-api/internal/models"
)

// ImportGames upserts with an explicit DoUpdates column list, so a field
// missing from it is silently ignored on re-import. That is not hypothetical:
// parent_igdb_id was missing, which meant a re-import dropped the parent it
// had just fetched. cmd/backfill re-imports the whole catalog, so both columns
// have to survive the conflict path.
func TestReimportUpdatesSummaryAndParent(t *testing.T) {
	app, tx := newTestApp(t)

	first := &models.Game{IGDBID: 903020, Title: "Some Game", Summary: "", ParentIGDBID: nil}
	if _, err := app.DB.ImportGames(context.Background(), []*models.Game{first}); err != nil {
		t.Fatalf("first import: %v", err)
	}

	parent := 903021
	again := &models.Game{IGDBID: 903020, Title: "Some Game", Summary: "Now described.", ParentIGDBID: &parent}
	if _, err := app.DB.ImportGames(context.Background(), []*models.Game{again}); err != nil {
		t.Fatalf("re-import: %v", err)
	}

	var stored models.Game
	if err := tx.Where("igdb_id = ?", 903020).First(&stored).Error; err != nil {
		t.Fatalf("read game: %v", err)
	}

	if stored.Summary != "Now described." {
		t.Errorf("summary = %q, want it updated by the re-import", stored.Summary)
	}
	if stored.ParentIGDBID == nil || *stored.ParentIGDBID != 903021 {
		t.Errorf("parentIgdbId = %v, want 903021 — the re-import dropped it", stored.ParentIGDBID)
	}
}

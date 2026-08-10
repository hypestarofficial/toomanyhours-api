package dbrepo

import (
	"context"
	"time"
	"toomanyhours-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GamesByIGDBIDs maps IGDB ids to local game ids, for the ones already known.
// Missing keys are the games that need importing.
func (m *PostgresDBRepo) GamesByIGDBIDs(ctx context.Context, igdbIDs []int) (map[int]int, error) {
	known := map[int]int{}
	if len(igdbIDs) == 0 {
		return known, nil
	}

	var rows []struct {
		ID     int
		IGDBID int `gorm:"column:igdb_id"`
	}

	err := m.GormDB.WithContext(ctx).
		Model(&models.Game{}).
		Select("id", "igdb_id").
		Where("igdb_id IN ?", igdbIDs).
		Find(&rows).Error

	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		known[r.IGDBID] = r.ID
	}
	return known, nil
}

// ImportGames upserts games and their tags, returning IGDB id to local id.
//
// One transaction for the lot: a partial import would leave a game with some of
// its tags, which no later read could detect as wrong.
//
// Idempotency is the database's job. ON CONFLICT on games.igdb_id and on
// tags(facet, igdb_id) means importing the same game twice updates it rather
// than duplicating it, with no pre-flight check and therefore no race window.
//
// DoUpdates rather than DoNothing, and not only to refresh stale data: GORM
// adds RETURNING for the primary key, and DO NOTHING returns no row on a
// conflict. The ID would come back zero, and the games_tags insert below would
// happily write a link to tag 0. DoUpdates always returns the row.
func (m *PostgresDBRepo) ImportGames(ctx context.Context, games []*models.Game) (map[int]int, error) {
	imported := map[int]int{}
	if len(games) == 0 {
		return imported, nil
	}

	err := m.GormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, g := range games {
			tags := g.Tags
			g.Tags = nil // associations are written explicitly below
			g.UpdatedAt = time.Now()

			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "igdb_id"}},
				// Every column IGDB owns. A field missing here is silently
				// ignored on re-import, which is how parent_igdb_id came to be
				// dropped by the very call that had just fetched it. Anything
				// added to models.Game from upstream belongs in this list.
				DoUpdates: clause.AssignmentColumns([]string{
					"title", "image", "kind", "release_date", "parent_igdb_id", "summary", "updated_at",
				}),
			}).Create(g).Error; err != nil {
				return err
			}

			tagIDs := make([]int, 0, len(tags))
			for _, t := range tags {
				t.UpdatedAt = time.Now()
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "facet"}, {Name: "igdb_id"}},
					DoUpdates: clause.AssignmentColumns([]string{"name", "updated_at"}),
				}).Create(t).Error; err != nil {
					return err
				}
				tagIDs = append(tagIDs, t.ID)
			}

			// Replace rather than append, so a game that lost a genre upstream
			// loses it here too.
			if err := tx.Exec("DELETE FROM games_tags WHERE game_id = ?", g.ID).Error; err != nil {
				return err
			}
			for _, tagID := range tagIDs {
				if err := tx.Exec(
					"INSERT INTO games_tags (game_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
					g.ID, tagID,
				).Error; err != nil {
					return err
				}
			}

			imported[g.IGDBID] = g.ID
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return imported, nil
}

-- Add-ons point at the game they belong to. IGDB's id, not ours, and no
-- foreign key: the parent is very often not in the catalog, because nothing
-- imports a game merely because something else references it. A reference
-- would reject the import of any add-on whose base game nobody has added.
ALTER TABLE public.games ADD COLUMN parent_igdb_id integer;

-- Backfill for the rows that already exist. SQL cannot call IGDB, so these are
-- literals — the same approach 000007 took with the seven games' real titles.
-- All four parents are already in the catalog, so these are known rather than
-- guessed.
--
-- Each is a no-op on a database that does not have that row, which is what
-- makes this safe on an empty one. A fresh database is the case that will
-- actually be run first in production; 000006 taught that by failing on it.
UPDATE public.games SET parent_igdb_id = 314246 WHERE igdb_id = 396307; -- Borderlands 4: Bounty Pack 2
UPDATE public.games SET parent_igdb_id = 314246 WHERE igdb_id = 396087; -- Borderlands 4: Story Pack 1
UPDATE public.games SET parent_igdb_id = 103292 WHERE igdb_id = 140517; -- Gears 5: Hivebusters
UPDATE public.games SET parent_igdb_id = 203722 WHERE igdb_id = 325582; -- Dave the Diver: In the Jungle

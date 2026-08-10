-- The column goes and the backfill goes with it. Nothing to preserve: every
-- value in it can be fetched from IGDB again.
ALTER TABLE public.games DROP COLUMN parent_igdb_id;

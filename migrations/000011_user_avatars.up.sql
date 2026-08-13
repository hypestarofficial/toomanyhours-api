-- A user's profile photo: one 256x256 JPEG, about 20KB.
--
-- Its own table rather than a column on users, because GORM selects every
-- column by default — an avatar on users would ride along on every login and
-- every profile read. user_id is the primary key because a user has at most one.
--
-- ON DELETE CASCADE, unlike user_games.game_id's RESTRICT: a photo belongs to
-- one account and is worth nothing without it.
--
-- No content_type column: the server always re-encodes to JPEG, so storing it
-- would be recording a constant.
CREATE TABLE public.user_avatars (
    user_id    integer PRIMARY KEY REFERENCES public.users (id) ON DELETE CASCADE,
    bytes      bytea       NOT NULL,
    -- The first 12 hex characters of the SHA-256, for cache busting. Not an
    -- integrity check.
    hash       text        NOT NULL,
    -- timestamptz, like refresh_tokens: any column Go does time arithmetic on
    -- gets the zone, or a process running in CEST writes values two hours
    -- ahead of the database's now().
    updated_at timestamptz NOT NULL DEFAULT now()
);

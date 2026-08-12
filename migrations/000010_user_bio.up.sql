-- A short self-description, shown on the public profile so a stranger reading
-- somebody's list knows whose taste they are reading.
--
-- Nullable with no default, like games.summary: a bio nobody has written is
-- absent, not empty, and an empty string would be a second way of saying the
-- same thing that every query then has to remember to check for.
--
-- char_length counts characters and Go's utf8.RuneCountInString counts runes,
-- which are the same thing — that is what makes this constraint and
-- validate.Bio agree rather than disagree at some boundary nobody tests.
ALTER TABLE public.users ADD COLUMN bio text;

ALTER TABLE public.users ADD CONSTRAINT users_bio_length
  CHECK (bio IS NULL OR char_length(bio) <= 500);

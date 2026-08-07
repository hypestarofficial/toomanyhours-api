-- The rule "usernames are lowercase" was real only in Go. internal/validate
-- lowercases before every write, but users_username_lower_idx is on
-- LOWER(username): it stops `Hype` colliding with `hype` and still allows
-- `Hype` to be stored. One forgetful code path was all it would take.
--
-- The UPDATE is a no-op on every database that exists today. It is here so the
-- constraint can be added to one that has drifted, and because a migration
-- that only works on tidy data is a migration that fails in production.
UPDATE public.users SET username = lower(username) WHERE username <> lower(username);

ALTER TABLE public.users
    ADD CONSTRAINT users_username_lowercase CHECK (username = lower(username));

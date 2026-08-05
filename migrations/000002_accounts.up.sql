-- Real accounts: profile visibility, and uniqueness that is actually enforced.
--
-- Until now the User struct carried `gorm:"unique"` tags that did nothing,
-- because the schema came from a SQL file and AutoMigrate was never called.
-- Two users could hold the same username. Usernames become public URLs at
-- /u/<username>, so this has to hold at the database level.

ALTER TABLE public.users ADD COLUMN visibility text NOT NULL DEFAULT 'public'
    CHECK (visibility IN ('public', 'private'));

ALTER TABLE public.users ALTER COLUMN username SET NOT NULL;
ALTER TABLE public.users ALTER COLUMN email    SET NOT NULL;
ALTER TABLE public.users ALTER COLUMN password SET NOT NULL;

-- Expression indexes on LOWER() rather than plain unique indexes. Application
-- code also lowercases before writing (internal/validate), so this is a
-- backstop: a future code path that forgets to normalize gets a constraint
-- violation instead of silently creating "Hype" alongside "hype".
CREATE UNIQUE INDEX users_username_lower_idx ON public.users (LOWER(username));
CREATE UNIQUE INDEX users_email_lower_idx    ON public.users (LOWER(email));

DROP INDEX IF EXISTS public.users_email_lower_idx;
DROP INDEX IF EXISTS public.users_username_lower_idx;

ALTER TABLE public.users ALTER COLUMN password DROP NOT NULL;
ALTER TABLE public.users ALTER COLUMN email    DROP NOT NULL;
ALTER TABLE public.users ALTER COLUMN username DROP NOT NULL;

ALTER TABLE public.users DROP COLUMN visibility;

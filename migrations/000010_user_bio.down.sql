ALTER TABLE public.users DROP CONSTRAINT IF EXISTS users_bio_length;
ALTER TABLE public.users DROP COLUMN IF EXISTS bio;

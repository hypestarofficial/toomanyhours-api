-- Only the constraint comes off. The UPDATE above cannot be reversed — there is
-- no record of what case a name originally had — and restoring it would not be
-- wanted anyway.
ALTER TABLE public.users DROP CONSTRAINT users_username_lowercase;

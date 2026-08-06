-- Lossy, unavoidably: narrowing to smallint rounds 6.5 to 7. That is inherent
-- to the type change rather than a flaw to work around, but it means rolling
-- back after anyone has used a half-step loses the half.
ALTER TABLE public.user_games DROP CONSTRAINT user_games_rating_check;

-- round() on numeric rounds half away from zero, so 0.5 becomes 1 and still
-- satisfies the restored BETWEEN. Without the explicit ::smallint the USING
-- expression yields numeric and the cast fails.
ALTER TABLE public.user_games
    ALTER COLUMN rating TYPE smallint USING round(rating)::smallint;

ALTER TABLE public.user_games
    ADD CONSTRAINT user_games_rating_check CHECK (rating BETWEEN 1 AND 10);

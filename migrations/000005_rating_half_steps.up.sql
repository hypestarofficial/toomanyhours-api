-- Ratings gain half-steps. The control was five stars doubled into an integer
-- 1-10, so a half star was 1 and 6.5 was unreachable; ten stars with half-steps
-- means a rating of 6.5 is stored as 6.5.
--
-- Existing values need no conversion. 8 meant four of five stars and now means
-- eight of ten - the same 80% of the bar, rendered identically.

-- The old CHECK goes first. `rating BETWEEN 1 AND 10` stays perfectly valid on
-- a numeric column, so leaving it in place would silently keep rejecting 0.5
-- while everything else looked converted.
ALTER TABLE public.user_games DROP CONSTRAINT user_games_rating_check;

ALTER TABLE public.user_games ALTER COLUMN rating TYPE numeric(3,1);

-- The half-step clause is what makes this more than a range check: numeric(3,1)
-- on its own accepts 6.3, which no control can produce and no reader expects.
-- NULL passes, so cleared ratings are unaffected.
ALTER TABLE public.user_games
    ADD CONSTRAINT user_games_rating_check
    CHECK (rating >= 0.5 AND rating <= 10 AND rating * 2 = trunc(rating * 2));

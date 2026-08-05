-- Dropped in dependency order: games_genres references both games and genres.
DROP TABLE IF EXISTS public.games_genres;
DROP TABLE IF EXISTS public.games;
DROP TABLE IF EXISTS public.genres;
DROP TABLE IF EXISTS public.users;

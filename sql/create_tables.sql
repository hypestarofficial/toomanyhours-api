-- PostgreSQL database dump
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

-- 1. Create Tables
CREATE TABLE public.genres (
    id integer NOT NULL GENERATED ALWAYS AS IDENTITY,
    genre character varying(255),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT genres_pkey PRIMARY KEY (id)
);

CREATE TABLE public.games (
    id integer NOT NULL GENERATED ALWAYS AS IDENTITY,
    title character varying(512),
    image character varying(255),
    release_date date,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT games_pkey PRIMARY KEY (id)
);

CREATE TABLE public.games_genres (
    id integer NOT NULL GENERATED ALWAYS AS IDENTITY,
    game_id integer,
    genre_id integer,
    CONSTRAINT games_genres_pkey PRIMARY KEY (id)
);

CREATE TABLE public.users (
    id integer NOT NULL GENERATED ALWAYS AS IDENTITY,
    username character varying(255),
    email character varying(255),
    password character varying(255),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT users_pkey PRIMARY KEY (id)
);

-- 2. Foreign Key Constraints
ALTER TABLE ONLY public.games_genres
    ADD CONSTRAINT games_genres_genre_id_fkey FOREIGN KEY (genre_id) REFERENCES public.genres(id) ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE ONLY public.games_genres
    ADD CONSTRAINT games_genres_game_id_fkey FOREIGN KEY (game_id) REFERENCES public.games(id) ON UPDATE CASCADE ON DELETE CASCADE;

-- 3. Seed Data (INSERT instead of COPY)
-- Using OVERRIDING SYSTEM VALUE because columns are GENERATED ALWAYS
INSERT INTO public.genres (id, genre, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'Singleplayer', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(2, 'Multiplayer', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(3, 'Story rich', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(4, 'RPG', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(5, 'Open World', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(6, 'Sandbox', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(7, 'FPS', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(8, 'Indie', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(9, 'Survival', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(10, 'Platformer', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(11, 'Racing', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(12, 'Fantasy', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(13, 'Superhero', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(14, 'Action-RPG', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(15, 'Sci-Fi', '2022-09-23 00:00:00', '2022-09-23 00:00:00');

INSERT INTO public.games (id, title, release_date, image, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(292030, 'The Witcher 3: Wild Hunt', '2015-05-19', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/292030/ad9240e088f953a84aee814034c50a6a92bf4516/header.jpg?t=1756366569', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(377160, 'Fallout 4', '2015-11-10', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/377160/header.jpg?t=1764687456', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(489830, 'The Elder Scrolls V: Skyrim Special Edition', '2011-11-11', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/72850/header.jpg?t=1756366569', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(1174180, 'Red Dead Redemption 2', '2018-10-26', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/1174180/header.jpg?t=1756366569', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(3240220, 'Grand Theft Auto V Enhanced', '2013-09-17', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/271590/header.jpg?t=1756366569', '2022-09-23 00:00:00', '2022-09-23 00:00:00');

INSERT INTO public.games_genres (id, game_id, genre_id) OVERRIDING SYSTEM VALUE VALUES
(1, 292030, 1), (2, 292030, 3), (3, 292030, 4), (4, 292030, 5), (5, 292030, 12),
(6, 377160, 1), (7, 377160, 3), (8, 377160, 14), (9, 377160, 15),
(10, 489830, 1), (11, 489830, 3), (12, 489830, 4), (13, 489830, 5), (14, 489830, 12),
(15, 1174180, 1), (16, 1174180, 3), (17, 1174180, 4),
(18, 3240220, 1), (19, 3240220, 3), (20, 3240220, 4);

INSERT INTO public.users (id, username, email, password, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'admin', 'admin@example.com', '$2a$14$wVsaPvJnJJsomWArouWCtusem6S/.Gauq/GjOIEHpyh2DAMmso1wy', '2022-09-23 00:00:00', '2022-09-23 00:00:00');

-- 4. Sync Sequences (Prevents "duplicate key" errors on next app insert)
SELECT setval('public.genres_id_seq', (SELECT MAX(id) FROM public.genres));
SELECT setval('public.games_id_seq', (SELECT MAX(id) FROM public.games));
SELECT setval('public.games_genres_id_seq', (SELECT MAX(id) FROM public.games_genres));
SELECT setval('public.users_id_seq', (SELECT MAX(id) FROM public.users));
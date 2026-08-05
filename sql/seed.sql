-- Local development seed data.
--
-- This file holds CONTENT only. Schema lives in ../migrations/ and is applied
-- with golang-migrate. Run this after migrations, against an empty database:
--
--     docker compose exec -T postgres psql -U toomanyhours -d toomanyhours < sql/seed.sql
--
-- The seeded admin account is a local fixture, not a real credential:
--     email:    admin@example.com
--     password: devpassword
-- Registration reserves the username "admin", so this row can only be created
-- here, by hand. Never deploy this file anywhere reachable from the internet.

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

-- Game IDs are Steam app IDs, supplied by the client rather than generated.
INSERT INTO public.games (id, title, release_date, image, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(730, 'Counter-Strike 2', '2026-01-01', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/730/header.jpg?t=1749053861', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(292030, 'The Witcher 3: Wild Hunt', '2015-05-19', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/292030/ad9240e088f953a84aee814034c50a6a92bf4516/header.jpg?t=1756366569', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(377160, 'Fallout 4', '2015-11-10', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/377160/header.jpg?t=1764687456', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(489830, 'The Elder Scrolls V: Skyrim Special Edition', '2011-11-11', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/72850/header.jpg?t=1756366569', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(1174180, 'Red Dead Redemption 2', '2018-10-26', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/1174180/header.jpg?t=1756366569', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(1395520, 'The Séance of Blake Manor', '2026-02-11', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/1395520/09df209c551539dc76679ca24b079b5771c2c281/header.jpg?t=1766141386', '2022-09-23 00:00:00', '2022-09-23 00:00:00'),
(3240220, 'Grand Theft Auto V Enhanced', '2013-09-17', 'https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/271590/header.jpg?t=1756366569', '2022-09-23 00:00:00', '2022-09-23 00:00:00');

-- No explicit ids: the identity column assigns them, which keeps the sequence
-- correct without a setval. Association rows are rewritten wholesale by
-- PostGameGenres, so their ids carry no meaning.
INSERT INTO public.games_genres (game_id, genre_id) VALUES
(730, 2), (730, 7),
(292030, 1), (292030, 3), (292030, 4), (292030, 5), (292030, 12),
(377160, 1), (377160, 3), (377160, 14), (377160, 15),
(489830, 1), (489830, 3), (489830, 4), (489830, 5), (489830, 12),
(1174180, 1), (1174180, 3), (1174180, 4),
(1395520, 1), (1395520, 8), (1395520, 12),
(3240220, 1), (3240220, 2), (3240220, 3), (3240220, 4), (3240220, 5);

INSERT INTO public.users (id, username, email, password, created_at, updated_at) OVERRIDING SYSTEM VALUE VALUES
(1, 'admin', 'admin@example.com', '$2a$12$O5uKxjg7sagbbNn3QF6yaOf7gkceN.7PGCecBqMAoHJRWuAxcGGSe', '2022-09-23 00:00:00', '2022-09-23 00:00:00');

-- Keep the identity sequences ahead of the explicit ids inserted above,
-- otherwise the next application insert collides on the primary key.
SELECT setval('public.genres_id_seq', (SELECT MAX(id) FROM public.genres));
SELECT setval('public.games_id_seq',  (SELECT MAX(id) FROM public.games));
SELECT setval('public.users_id_seq',  (SELECT MAX(id) FROM public.users));

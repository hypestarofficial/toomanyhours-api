--
-- PostgreSQL database dump
--

-- Dumped from database version 14.5 (Debian 14.5-1.pgdg110+1)
-- Dumped by pg_dump version 14.5 (Homebrew)

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

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: genres; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.genres (
    id integer NOT NULL,
    genre character varying(255),
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: genres_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.genres ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.genres_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: games; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.games (
    id integer NOT NULL,
    title character varying(512),
    image character varying(255),
    release_date date,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: games_genres; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.games_genres (
    id integer NOT NULL,
    game_id integer,
    genre_id integer
);


--
-- Name: games_genres_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.games_genres ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.games_genres_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: games_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.games ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.games_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    username character varying(255),
    email character varying(255),
    password character varying(255),
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Data for Name: genres; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.genres (id, genre, created_at, updated_at) FROM stdin;
1	Comedy	2022-09-23 00:00:00	2022-09-23 00:00:00
2	Sci-Fi	2022-09-23 00:00:00	2022-09-23 00:00:00
3	Horror	2022-09-23 00:00:00	2022-09-23 00:00:00
4	Romance	2022-09-23 00:00:00	2022-09-23 00:00:00
5	Action	2022-09-23 00:00:00	2022-09-23 00:00:00
6	Thriller	2022-09-23 00:00:00	2022-09-23 00:00:00
7	Drama	2022-09-23 00:00:00	2022-09-23 00:00:00
8	Mystery	2022-09-23 00:00:00	2022-09-23 00:00:00
9	Crime	2022-09-23 00:00:00	2022-09-23 00:00:00
10	Animation	2022-09-23 00:00:00	2022-09-23 00:00:00
11	Adventure	2022-09-23 00:00:00	2022-09-23 00:00:00
12	Fantasy	2022-09-23 00:00:00	2022-09-23 00:00:00
13	Superhero	2022-09-23 00:00:00	2022-09-23 00:00:00
\.


--
-- Data for Name: games; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.games (id, title, release_date, image, created_at, updated_at) FROM stdin;
1	Witcher 3: Wild Hunt	2015-05-19	https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/292030/ad9240e088f953a84aee814034c50a6a92bf4516/header.jpg?t=1756366569	2022-09-23 00:00:00	2022-09-23 00:00:00
2	Fallout 4	2015-11-10	https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/377160/header.jpg?t=1764687456	2022-09-23 00:00:00	2022-09-23 00:00:00
3	Skyrim	2011-11-11	https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/72850/header.jpg?t=1756366569	2022-09-23 00:00:00	2022-09-23 00:00:00
4	Red Dead Redemption 2	2018-10-26	https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/1174180/header.jpg?t=1756366569	2022-09-23 00:00:00	2022-09-23 00:00:00
5	Grand Theft Auto V	2013-09-17	https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/271590/header.jpg?t=1756366569	2022-09-23 00:00:00	2022-09-23 00:00:00
\.


--
-- Data for Name: games_genres; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.games_genres (id, game_id, genre_id) FROM stdin;
1	1	5
2	1	12
3	2	5
4	2	11
5	3	9
6	3	7
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.users (id, username, email, password, created_at, updated_at) FROM stdin;
1	admin	admin@example.com	$2a$14$wVsaPvJnJJsomWArouWCtusem6S/.Gauq/GjOIEHpyh2DAMmso1wy	2022-09-23 00:00:00	2022-09-23 00:00:00
\.


--
-- Name: genres_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.genres_id_seq', 13, true);


--
-- Name: games_genres_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.games_genres_id_seq', 6, true);


--
-- Name: games_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.games_id_seq', 3, true);


--
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.users_id_seq', 1, true);


--
-- Name: genres genres_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.genres
    ADD CONSTRAINT genres_pkey PRIMARY KEY (id);


--
-- Name: games_genres games_genres_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.games_genres
    ADD CONSTRAINT games_genres_pkey PRIMARY KEY (id);


--
-- Name: games games_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.games
    ADD CONSTRAINT games_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: games_genres games_genres_genre_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.games_genres
    ADD CONSTRAINT games_genres_genre_id_fkey FOREIGN KEY (genre_id) REFERENCES public.genres(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: games_genres games_genres_game_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.games_genres
    ADD CONSTRAINT games_genres_game_id_fkey FOREIGN KEY (game_id) REFERENCES public.games(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


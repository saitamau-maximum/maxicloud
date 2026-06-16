--
-- PostgreSQL database dump
--

\restrict ToRBlKtkmDPraVQlRSsOLWCY9xTGaoQoRxMXHf1ADIkfHDXaoObYSps7Hdbc99B

-- Dumped from database version 15.18 (Debian 15.18-1.pgdg13+1)
-- Dumped by pg_dump version 18.4

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
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
-- Name: deployment_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.deployment_histories (
    id text NOT NULL,
    application_id text NOT NULL,
    owner_user_id text NOT NULL,
    repo_owner text NOT NULL,
    repo_name text NOT NULL,
    commit_sha text NOT NULL,
    commit_message text DEFAULT ''::text NOT NULL,
    commit_author text DEFAULT ''::text NOT NULL,
    commit_at timestamp with time zone NOT NULL,
    pr_number integer,
    status text NOT NULL,
    started_at timestamp with time zone NOT NULL,
    finished_at timestamp with time zone,
    CONSTRAINT finished_after_started CHECK (((finished_at IS NULL) OR (finished_at >= started_at))),
    CONSTRAINT pr_number_positive CHECK (((pr_number IS NULL) OR (pr_number > 0))),
    CONSTRAINT status_check CHECK ((status = ANY (ARRAY['QUEUED'::text, 'IN_PROGRESS'::text, 'SUCCESS'::text, 'FAILED'::text])))
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id text NOT NULL,
    display_id text NOT NULL,
    display_name text NOT NULL,
    roles text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: deployment_histories deployment_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.deployment_histories
    ADD CONSTRAINT deployment_histories_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: deployment_histories_application_id_started_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX deployment_histories_application_id_started_at_idx ON public.deployment_histories USING btree (application_id, started_at DESC);


--
-- Name: deployment_histories_owner_user_id_started_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX deployment_histories_owner_user_id_started_at_idx ON public.deployment_histories USING btree (owner_user_id, started_at DESC);


--
-- Name: deployment_histories_repo_owner_repo_name_commit_sha_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX deployment_histories_repo_owner_repo_name_commit_sha_idx ON public.deployment_histories USING btree (repo_owner, repo_name, commit_sha);


--
-- PostgreSQL database dump complete
--

\unrestrict ToRBlKtkmDPraVQlRSsOLWCY9xTGaoQoRxMXHf1ADIkfHDXaoObYSps7Hdbc99B


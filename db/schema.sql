--
-- PostgreSQL database dump
--

\restrict bK4L4Gyfb1EpXJVScxWxCntmwAh3KPuCNienQgGxlW80gJ7ugr5H9lvekCatlfY

-- Dumped from database version 15.19 (Debian 15.19-1.pgdg13+2)
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
-- Name: project_group_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.project_group_roles (
    id text NOT NULL,
    project_id text NOT NULL,
    oidc_role text NOT NULL,
    role text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by text NOT NULL,
    CONSTRAINT project_group_roles_role_check CHECK ((role = ANY (ARRAY['editor'::text, 'admin'::text])))
);


--
-- Name: project_members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.project_members (
    id text NOT NULL,
    project_id text NOT NULL,
    user_id text NOT NULL,
    role text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by text NOT NULL,
    CONSTRAINT project_members_role_check CHECK ((role = ANY (ARRAY['editor'::text, 'admin'::text])))
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
-- Name: project_group_roles project_group_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_group_roles
    ADD CONSTRAINT project_group_roles_pkey PRIMARY KEY (id);


--
-- Name: project_group_roles project_group_roles_project_id_oidc_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_group_roles
    ADD CONSTRAINT project_group_roles_project_id_oidc_role_key UNIQUE (project_id, oidc_role);


--
-- Name: project_members project_members_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_members
    ADD CONSTRAINT project_members_pkey PRIMARY KEY (id);


--
-- Name: project_members project_members_project_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_members
    ADD CONSTRAINT project_members_project_id_user_id_key UNIQUE (project_id, user_id);


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
-- Name: idx_project_group_roles_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_group_roles_project_id ON public.project_group_roles USING btree (project_id);


--
-- Name: idx_project_members_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_members_project_id ON public.project_members USING btree (project_id);


--
-- PostgreSQL database dump complete
--

\unrestrict bK4L4Gyfb1EpXJVScxWxCntmwAh3KPuCNienQgGxlW80gJ7ugr5H9lvekCatlfY


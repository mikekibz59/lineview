-- Runs as dev_user (superuser) connected to lineview_db
--
-- dev_user : superuser + owner of every table. Used for DDL and the GO loader.
--            It Bypasses RLS, so never test isolation with it.
-- lineview_app: what the application (and RLS test) connects as.
--               Not a superuser, not the table owner, no BYPASSRLS, so policies
--               actually apply to it.
CREATE ROLE lineview_app LOGIN PASSWORD 'dev_app' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;

GRANT CONNECT ON DATABASE lineview_db TO lineview_app;
GRANT USAGE ON SCHEMA public TO lineview_app;

-- Tables are created later (02-schema.sql) by dev_user. Default privileges make every table dev_user
-- creates in public readable/writable by the app role, so you can start testing without a grants file.
ALTER DEFAULT PRIVILEGES FOR ROLE dev_user IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO lineview_app;

-- lineview_auth: resolves a session token to (tenant, user) BEFORE any tenant context exists, so it
-- cannot be filtered by RLS. It is kept narrow on purpose: it can read the auth schema and nothing else (no factory tables).

CREATE ROLE lineview_auth LOGIN PASSWORD 'dev_auth' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
GRANT CONNECT ON DATABASE lineview_db TO lineview_auth;

-- Mock authentication: users and pre-seeded sessions (written by the loader).
-- No passwords, no signup. The subject under test is tenant isolation.
--
-- Lives in its own schema so lineview_app (which has default privileges only on `public`) cannot see it.
-- Only lineview_auth can.

CREATE SCHEMA auth;

CREATE TABLE auth.users (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    id uuid NOT NULL DEFAULT uuidv7(),
    email text NOT NULL,
    display_name text NOT NULL,
    created_at text NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, email)
);


CREATE TABLE auth.sessions(
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, user_id) REFERENCES auth.users (tenant_id, id)
);

GRANT USAGE ON SCHEMA auth to lineview_auth;
GRANT SELECT ON auth.users, auth.sessions TO lineview_auth;

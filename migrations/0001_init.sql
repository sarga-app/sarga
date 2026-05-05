-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE users (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username            TEXT        NOT NULL,
    email               TEXT,
    password_hash       TEXT,
    display_name        TEXT,
    is_admin            BOOLEAN     NOT NULL DEFAULT FALSE,
    disabled_at         TIMESTAMPTZ,
    password_changed_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_username_lowercase CHECK (username = lower(username)),
    CONSTRAINT users_email_lowercase    CHECK (email IS NULL OR email = lower(email))
);
CREATE UNIQUE INDEX users_username_key ON users (username);
CREATE UNIQUE INDEX users_email_key    ON users (email) WHERE email IS NOT NULL;

CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE oidc_identities (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer         TEXT        NOT NULL,
    subject        TEXT        NOT NULL,
    email_at_link  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX oidc_identities_issuer_subject_key ON oidc_identities (issuer, subject);
CREATE INDEX        oidc_identities_user_id_idx        ON oidc_identities (user_id);

CREATE TABLE refresh_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  BYTEA       NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    parent_id   UUID        REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT refresh_tokens_token_hash_len CHECK (octet_length(token_hash) = 32)
);
CREATE UNIQUE INDEX refresh_tokens_token_hash_key ON refresh_tokens (token_hash);
CREATE INDEX        refresh_tokens_user_id_idx    ON refresh_tokens (user_id);
CREATE INDEX        refresh_tokens_parent_id_idx  ON refresh_tokens (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX        refresh_tokens_gc_idx         ON refresh_tokens (expires_at) WHERE revoked_at IS NULL;

CREATE TABLE libraries (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    path       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX libraries_name_key ON libraries (name);

CREATE TRIGGER libraries_updated_at BEFORE UPDATE ON libraries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE library_access (
    library_id UUID        NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id)     ON DELETE CASCADE,
    role       TEXT        NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (library_id, user_id),
    CONSTRAINT library_access_role_check CHECK (role IN ('viewer', 'contributor', 'admin'))
);
CREATE INDEX library_access_user_id_idx ON library_access (user_id);


-- +goose Down

DROP TABLE IF EXISTS library_access;
DROP TABLE IF EXISTS libraries;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS oidc_identities;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();

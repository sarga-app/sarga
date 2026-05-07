-- +goose Up

CREATE TABLE users (
    id                  TEXT    PRIMARY KEY,
    username            TEXT    NOT NULL,
    email               TEXT,
    password_hash       TEXT,
    display_name        TEXT,
    is_admin            INTEGER NOT NULL DEFAULT 0,
    disabled_at         TEXT,
    password_changed_at TEXT,
    created_at          TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (username = lower(username)),
    CHECK (email IS NULL OR email = lower(email))
) STRICT;
CREATE UNIQUE INDEX users_username_key ON users (username);
CREATE UNIQUE INDEX users_email_key    ON users (email) WHERE email IS NOT NULL;

CREATE TABLE oidc_identities (
    id            TEXT NOT NULL PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer        TEXT NOT NULL,
    subject       TEXT NOT NULL,
    email_at_link TEXT,
    created_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;
CREATE UNIQUE INDEX oidc_identities_issuer_subject_key ON oidc_identities (issuer, subject);
CREATE INDEX        oidc_identities_user_id_idx        ON oidc_identities (user_id);

CREATE TABLE refresh_tokens (
    id         TEXT NOT NULL PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    parent_id  TEXT REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(token_hash) = 32)
) STRICT;
CREATE UNIQUE INDEX refresh_tokens_token_hash_key ON refresh_tokens (token_hash);
CREATE INDEX        refresh_tokens_user_id_idx    ON refresh_tokens (user_id);
CREATE INDEX        refresh_tokens_parent_id_idx  ON refresh_tokens (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX        refresh_tokens_gc_idx         ON refresh_tokens (expires_at) WHERE revoked_at IS NULL;

CREATE TABLE libraries (
    id         TEXT NOT NULL PRIMARY KEY,
    name       TEXT NOT NULL,
    path       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;
CREATE UNIQUE INDEX libraries_name_key ON libraries (name);

CREATE TABLE library_access (
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id)     ON DELETE CASCADE,
    role       TEXT NOT NULL,
    granted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (library_id, user_id),
    CHECK (role IN ('viewer', 'contributor', 'admin'))
) STRICT, WITHOUT ROWID;
CREATE INDEX library_access_user_id_idx ON library_access (user_id);


-- +goose Down

DROP TABLE IF EXISTS library_access;
DROP TABLE IF EXISTS libraries;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS oidc_identities;
DROP TABLE IF EXISTS users;

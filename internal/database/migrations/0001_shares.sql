CREATE TABLE shares (
    id                   TEXT    PRIMARY KEY NOT NULL CHECK (length(id) = 22),
    payload_kind         TEXT    NOT NULL CHECK (payload_kind IN ('TEXT', 'FILE')),
    privacy_mode         TEXT    NOT NULL CHECK (privacy_mode IN ('STANDARD', 'ENCRYPTED')),
    owner_token_verifier BLOB    NOT NULL CHECK (typeof(owner_token_verifier) = 'blob' AND length(owner_token_verifier) = 32),
    created_at           INTEGER NOT NULL,
    updated_at           INTEGER NOT NULL,
    expires_at           INTEGER,
    CHECK (payload_kind != 'FILE' OR privacy_mode = 'STANDARD')
) STRICT;

CREATE INDEX shares_expires_at_idx ON shares(expires_at) WHERE expires_at IS NOT NULL;

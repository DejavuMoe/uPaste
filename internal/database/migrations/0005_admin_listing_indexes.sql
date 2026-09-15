CREATE INDEX shares_created_at_idx ON shares(created_at);

CREATE INDEX shares_payload_privacy_created_idx
    ON shares(payload_kind, privacy_mode, created_at);

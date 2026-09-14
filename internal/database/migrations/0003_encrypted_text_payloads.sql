CREATE TABLE encrypted_text_payloads (
    share_id TEXT PRIMARY KEY NOT NULL
        REFERENCES shares(id) ON DELETE CASCADE,
    protocol TEXT NOT NULL
        CHECK (protocol = 'UPASTE_AES_GCM_V1'),
    nonce BLOB NOT NULL
        CHECK (typeof(nonce) = 'blob' AND length(nonce) = 12),
    ciphertext BLOB NOT NULL
        CHECK (typeof(ciphertext) = 'blob' AND length(ciphertext) BETWEEN 19 AND 1048594)
) STRICT;

CREATE TRIGGER standard_text_payload_privacy_insert
BEFORE INSERT ON standard_text_payloads
WHEN NOT EXISTS (
    SELECT 1 FROM shares
    WHERE id = NEW.share_id AND payload_kind = 'TEXT' AND privacy_mode = 'STANDARD'
)
BEGIN
    SELECT RAISE(ABORT, 'standard text payload requires TEXT + STANDARD Share');
END;

CREATE TRIGGER standard_text_payload_privacy_update
BEFORE UPDATE OF share_id ON standard_text_payloads
WHEN NOT EXISTS (
    SELECT 1 FROM shares
    WHERE id = NEW.share_id AND payload_kind = 'TEXT' AND privacy_mode = 'STANDARD'
)
BEGIN
    SELECT RAISE(ABORT, 'standard text payload requires TEXT + STANDARD Share');
END;

CREATE TRIGGER encrypted_text_payload_privacy_insert
BEFORE INSERT ON encrypted_text_payloads
WHEN NOT EXISTS (
    SELECT 1 FROM shares
    WHERE id = NEW.share_id AND payload_kind = 'TEXT' AND privacy_mode = 'ENCRYPTED'
)
BEGIN
    SELECT RAISE(ABORT, 'encrypted text payload requires TEXT + ENCRYPTED Share');
END;

CREATE TRIGGER encrypted_text_payload_privacy_update
BEFORE UPDATE OF share_id ON encrypted_text_payloads
WHEN NOT EXISTS (
    SELECT 1 FROM shares
    WHERE id = NEW.share_id AND payload_kind = 'TEXT' AND privacy_mode = 'ENCRYPTED'
)
BEGIN
    SELECT RAISE(ABORT, 'encrypted text payload requires TEXT + ENCRYPTED Share');
END;

CREATE TRIGGER share_payload_privacy_update
BEFORE UPDATE OF payload_kind, privacy_mode ON shares
WHEN (
    EXISTS (SELECT 1 FROM standard_text_payloads WHERE share_id = OLD.id)
    AND (NEW.payload_kind != 'TEXT' OR NEW.privacy_mode != 'STANDARD')
) OR (
    EXISTS (SELECT 1 FROM encrypted_text_payloads WHERE share_id = OLD.id)
    AND (NEW.payload_kind != 'TEXT' OR NEW.privacy_mode != 'ENCRYPTED')
)
BEGIN
    SELECT RAISE(ABORT, 'Share type/privacy does not match payload');
END;

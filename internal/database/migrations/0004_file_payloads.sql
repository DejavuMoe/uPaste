CREATE TABLE file_payloads (
    share_id TEXT PRIMARY KEY NOT NULL
        REFERENCES shares(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL UNIQUE
        CHECK (length(storage_key) = 32 AND storage_key NOT GLOB '*[^0-9a-f]*'),
    original_filename TEXT NOT NULL
        CHECK (length(CAST(original_filename AS BLOB)) BETWEEN 1 AND 255),
    size_bytes INTEGER NOT NULL
        CHECK (size_bytes BETWEEN 1 AND 67108864),
    detected_media_type TEXT NOT NULL
        CHECK (length(detected_media_type) BETWEEN 1 AND 255),
    content_sha256 BLOB NOT NULL
        CHECK (typeof(content_sha256) = 'blob' AND length(content_sha256) = 32)
) STRICT;

CREATE TRIGGER file_payload_privacy_insert
BEFORE INSERT ON file_payloads
WHEN NOT EXISTS (
    SELECT 1 FROM shares
    WHERE id = NEW.share_id AND payload_kind = 'FILE' AND privacy_mode = 'STANDARD'
)
BEGIN
    SELECT RAISE(ABORT, 'file payload requires FILE + STANDARD Share');
END;

CREATE TRIGGER file_payload_privacy_update
BEFORE UPDATE OF share_id ON file_payloads
WHEN NOT EXISTS (
    SELECT 1 FROM shares
    WHERE id = NEW.share_id AND payload_kind = 'FILE' AND privacy_mode = 'STANDARD'
)
BEGIN
    SELECT RAISE(ABORT, 'file payload requires FILE + STANDARD Share');
END;

CREATE TRIGGER file_share_payload_privacy_update
BEFORE UPDATE OF payload_kind, privacy_mode ON shares
WHEN EXISTS (SELECT 1 FROM file_payloads WHERE share_id = OLD.id)
 AND (NEW.payload_kind != 'FILE' OR NEW.privacy_mode != 'STANDARD')
BEGIN
    SELECT RAISE(ABORT, 'Share type/privacy does not match file payload');
END;

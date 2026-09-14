CREATE TABLE standard_text_payloads (
    share_id TEXT PRIMARY KEY NOT NULL
        REFERENCES shares(id) ON DELETE CASCADE,
    format TEXT NOT NULL
        CHECK (format IN ('PLAIN', 'SOURCE', 'MARKDOWN')),
    content TEXT NOT NULL
        CHECK (length(CAST(content AS BLOB)) BETWEEN 1 AND 1048576)
) STRICT;

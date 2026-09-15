package share

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/domain"
)

// AdminFilter selects a bounded, cursor-paginated admin listing.
type AdminFilter struct {
	PayloadKind domain.PayloadKind
	PrivacyMode domain.PrivacyMode
	Lifecycle   string // "active", "expired", or ""
	Sort        string // "newest" (default) or "oldest"
	Cursor      string
	Limit       int
	ExactID     capability.ShareID
}

// AdminSummary is a small governance summary. It never includes capability or
// key material.
type AdminSummary struct {
	Active    int64 `json:"active"`
	Expired   int64 `json:"expired"`
	Text      int64 `json:"text"`
	File      int64 `json:"file"`
	Encrypted int64 `json:"encrypted"`
	FileBytes int64 `json:"file_bytes"`
}

// AdminList returns a bounded page and an opaque next cursor.
func (service *Service) AdminList(ctx context.Context, filter AdminFilter) ([]Share, string, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	now := service.serverNow().UnixMilli()
	params := make([]any, 0, 8)
	clauses := make([]string, 0, 6)

	if filter.ExactID.String() != "" {
		clauses = append(clauses, "s.id = ?")
		params = append(params, filter.ExactID.String())
	}
	if filter.PayloadKind != "" {
		clauses = append(clauses, "s.payload_kind = ?")
		params = append(params, string(filter.PayloadKind))
	}
	if filter.PrivacyMode != "" {
		clauses = append(clauses, "s.privacy_mode = ?")
		params = append(params, string(filter.PrivacyMode))
	}
	switch filter.Lifecycle {
	case "active":
		clauses = append(clauses, "(s.expires_at IS NULL OR s.expires_at > ?)")
		params = append(params, now)
	case "expired":
		clauses = append(clauses, "(s.expires_at IS NOT NULL AND s.expires_at <= ?)")
		params = append(params, now)
	case "":
	default:
		return nil, "", fmt.Errorf("invalid lifecycle filter")
	}

	order := "DESC"
	comparison := "<"
	if strings.EqualFold(filter.Sort, "oldest") {
		order = "ASC"
		comparison = ">"
	} else if filter.Sort != "" && !strings.EqualFold(filter.Sort, "newest") {
		return nil, "", fmt.Errorf("invalid sort")
	}

	cursorCreatedAt, cursorID, err := decodeAdminCursor(filter.Cursor)
	if err != nil {
		return nil, "", err
	}
	if filter.Cursor != "" {
		clauses = append(clauses, fmt.Sprintf("(s.created_at %s ? OR (s.created_at = ? AND s.id %s ?))", comparison, comparison))
		params = append(params, cursorCreatedAt, cursorCreatedAt, cursorID)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	query := `
SELECT s.id, s.payload_kind, s.privacy_mode, s.created_at, s.updated_at, s.expires_at,
       p.format, p.content, e.protocol, e.nonce, e.ciphertext,
       f.storage_key, f.original_filename, f.size_bytes, f.detected_media_type, f.content_sha256
FROM shares s
LEFT JOIN standard_text_payloads p ON p.share_id = s.id
LEFT JOIN encrypted_text_payloads e ON e.share_id = s.id
LEFT JOIN file_payloads f ON f.share_id = s.id
` + where + fmt.Sprintf(" ORDER BY s.created_at %s, s.id %s LIMIT ?", order, order)
	params = append(params, filter.Limit+1)

	rows, err := service.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, "", fmt.Errorf("admin list shares: %w", err)
	}
	defer rows.Close()
	values := make([]Share, 0, filter.Limit)
	hasMore := false
	for rows.Next() {
		if len(values) == filter.Limit {
			hasMore = true
			break
		}
		value, err := scanAdminShare(rows)
		if err != nil {
			return nil, "", err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if hasMore && len(values) > 0 {
		last := values[len(values)-1]
		next = encodeAdminCursor(last.CreatedAt.UnixMilli(), last.ID.String())
	}
	return values, next, nil
}

// AdminGet returns a Share even if it is expired.
func (service *Service) AdminGet(ctx context.Context, id capability.ShareID) (Share, error) {
	return loadActive(ctx, service.db, id, time.Time{})
}

// AdminSummary returns small aggregate counts and stored File bytes.
func (service *Service) AdminSummary(ctx context.Context) (AdminSummary, error) {
	now := service.serverNow().UnixMilli()
	var summary AdminSummary
	err := service.db.QueryRowContext(ctx, `
SELECT
COALESCE(SUM(CASE WHEN expires_at IS NULL OR expires_at > ? THEN 1 ELSE 0 END), 0),
COALESCE(SUM(CASE WHEN expires_at IS NOT NULL AND expires_at <= ? THEN 1 ELSE 0 END), 0),
COALESCE(SUM(CASE WHEN payload_kind = 'TEXT' THEN 1 ELSE 0 END), 0),
COALESCE(SUM(CASE WHEN payload_kind = 'FILE' THEN 1 ELSE 0 END), 0),
COALESCE(SUM(CASE WHEN privacy_mode = 'ENCRYPTED' THEN 1 ELSE 0 END), 0)
FROM shares
`, now, now).Scan(&summary.Active, &summary.Expired, &summary.Text, &summary.File, &summary.Encrypted)
	if err != nil {
		return AdminSummary{}, fmt.Errorf("admin summary: %w", err)
	}
	if err := service.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM file_payloads`).Scan(&summary.FileBytes); err != nil {
		return AdminSummary{}, fmt.Errorf("admin file bytes: %w", err)
	}
	return summary, nil
}

// AdminDelete removes any Share, including expired content, using the same
// cascade and File-object cleanup semantics as owner deletion.
func (service *Service) AdminDelete(ctx context.Context, id capability.ShareID) error {
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin admin delete: %w", err)
	}
	defer tx.Rollback()
	var storageKey sql.NullString
	var kind string
	if err := tx.QueryRowContext(ctx, `
SELECT payload_kind, f.storage_key
FROM shares s LEFT JOIN file_payloads f ON f.share_id = s.id
WHERE s.id = ?
`, id.String()).Scan(&kind, &storageKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load admin delete target: %w", err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM shares WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("admin delete Share: %w", err)
	}
	if err := requireOneRow(result, "admin delete Share"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit admin delete: %w", err)
	}
	if !storageKey.Valid || storageKey.String == "" {
		return nil
	}
	if service.store == nil {
		return &CleanupError{StorageKey: storageKey.String, Err: errors.New("object storage is unavailable")}
	}
	if err := service.store.Delete(storageKey.String); err != nil {
		return &CleanupError{StorageKey: storageKey.String, Err: err}
	}
	return nil
}

func scanAdminShare(rows *sql.Rows) (Share, error) {
	var value Share
	var idValue, kind, privacy string
	var createdAt, updatedAt int64
	var expiresAt, fileSize sql.NullInt64
	var format, content, protocol, storageKey, filename, mediaType sql.NullString
	var nonce, ciphertext, fileHash []byte
	if err := rows.Scan(
		&idValue, &kind, &privacy, &createdAt, &updatedAt, &expiresAt,
		&format, &content, &protocol, &nonce, &ciphertext,
		&storageKey, &filename, &fileSize, &mediaType, &fileHash,
	); err != nil {
		return Share{}, fmt.Errorf("scan admin Share: %w", err)
	}
	parsedID, err := capability.ParseShareID(idValue)
	if err != nil {
		return Share{}, fmt.Errorf("admin Share ID: %w", err)
	}
	value.ID = parsedID
	value.PayloadKind, err = domain.ParsePayloadKind(kind)
	if err != nil {
		return Share{}, err
	}
	value.PrivacyMode, err = domain.ParsePrivacyMode(privacy)
	if err != nil {
		return Share{}, err
	}
	value.CreatedAt = time.UnixMilli(createdAt).UTC()
	value.UpdatedAt = time.UnixMilli(updatedAt).UTC()
	value.ExpiresAt = expirationTime(expiresAt)
	switch value.PayloadKind {
	case domain.PayloadText:
		if value.PrivacyMode == domain.PrivacyStandard {
			if !format.Valid || !content.Valid {
				return Share{}, ErrPayloadMismatch
			}
			textFormat, err := domain.ParseTextFormat(format.String)
			if err != nil {
				return Share{}, err
			}
			value.Text = &Text{Format: textFormat, Content: content.String}
		} else {
			if !protocol.Valid {
				return Share{}, ErrPayloadMismatch
			}
			value.EncryptedText = &EncryptedText{Protocol: protocol.String, Nonce: nonce, Ciphertext: ciphertext}
		}
	case domain.PayloadFile:
		if !storageKey.Valid || !filename.Valid || !fileSize.Valid || !mediaType.Valid || len(fileHash) != 32 {
			return Share{}, ErrPayloadMismatch
		}
		value.File = &File{StorageKey: storageKey.String, Filename: filename.String, Size: fileSize.Int64, MediaType: mediaType.String}
		copy(value.File.SHA256[:], fileHash)
	default:
		return Share{}, ErrPayloadMismatch
	}
	return value, nil
}

func encodeAdminCursor(createdAt int64, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(createdAt, 10) + "|" + id))
}

func decodeAdminCursor(cursor string) (int64, string, error) {
	if cursor == "" {
		return 0, "", nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", errors.New("invalid admin cursor")
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return 0, "", errors.New("invalid admin cursor")
	}
	createdAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", errors.New("invalid admin cursor")
	}
	if _, err := capability.ParseShareID(parts[1]); err != nil {
		return 0, "", errors.New("invalid admin cursor")
	}
	return createdAt, parts[1], nil
}

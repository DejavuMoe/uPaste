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

// adminListQuery is deliberately payload-free: it computes byte counts and
// lightweight File metadata in SQLite without selecting TEXT/BLOB payload
// content, ciphertext, nonce, storage keys, hashes, or verifiers.
const adminListQuery = `
SELECT s.id, s.payload_kind, s.privacy_mode, s.created_at, s.updated_at, s.expires_at,
       CASE
         WHEN s.payload_kind = 'TEXT' AND s.privacy_mode = 'STANDARD' THEN length(CAST(p.content AS BLOB))
         WHEN s.payload_kind = 'TEXT' AND s.privacy_mode = 'ENCRYPTED' THEN length(e.ciphertext)
         WHEN s.payload_kind = 'FILE' THEN f.size_bytes
       END AS payload_bytes,
       f.original_filename, f.detected_media_type
FROM shares s
LEFT JOIN standard_text_payloads p ON p.share_id = s.id
LEFT JOIN encrypted_text_payloads e ON e.share_id = s.id
LEFT JOIN file_payloads f ON f.share_id = s.id
`

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

// AdminListItem is a lightweight governance listing row. It deliberately
// contains no payload content, ciphertext, nonce, storage key, or verifier.
type AdminListItem struct {
	ID            capability.ShareID
	PayloadKind   domain.PayloadKind
	PrivacyMode   domain.PrivacyMode
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     *time.Time
	PayloadBytes  int64
	FileFilename  string
	FileMediaType string
}

// AdminList returns a bounded page of metadata-only rows and an opaque cursor.
func (service *Service) AdminList(ctx context.Context, filter AdminFilter) ([]AdminListItem, string, error) {
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
	// Compute only byte counts and lightweight File metadata in SQLite. Payload
	// TEXT/BLOB columns are never selected into the Go process here.
	query := adminListQuery + where + fmt.Sprintf(" ORDER BY s.created_at %s, s.id %s LIMIT ?", order, order)
	params = append(params, filter.Limit+1)

	rows, err := service.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, "", fmt.Errorf("admin list shares: %w", err)
	}
	defer rows.Close()
	values := make([]AdminListItem, 0, filter.Limit)
	hasMore := false
	for rows.Next() {
		if len(values) == filter.Limit {
			hasMore = true
			break
		}
		item, err := scanAdminListItem(rows)
		if err != nil {
			return nil, "", err
		}
		values = append(values, item)
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

func scanAdminListItem(rows *sql.Rows) (AdminListItem, error) {
	var item AdminListItem
	var idValue, kind, privacy string
	var createdAt, updatedAt int64
	var expiresAt, payloadBytes sql.NullInt64
	var filename, mediaType sql.NullString
	if err := rows.Scan(
		&idValue, &kind, &privacy, &createdAt, &updatedAt, &expiresAt,
		&payloadBytes, &filename, &mediaType,
	); err != nil {
		return AdminListItem{}, fmt.Errorf("scan admin list item: %w", err)
	}
	parsedID, err := capability.ParseShareID(idValue)
	if err != nil {
		return AdminListItem{}, fmt.Errorf("admin list Share ID: %w", err)
	}
	item.ID = parsedID
	item.PayloadKind, err = domain.ParsePayloadKind(kind)
	if err != nil {
		return AdminListItem{}, fmt.Errorf("admin list payload kind: %w", err)
	}
	item.PrivacyMode, err = domain.ParsePrivacyMode(privacy)
	if err != nil {
		return AdminListItem{}, fmt.Errorf("admin list privacy mode: %w", err)
	}
	item.CreatedAt = time.UnixMilli(createdAt).UTC()
	item.UpdatedAt = time.UnixMilli(updatedAt).UTC()
	item.ExpiresAt = expirationTime(expiresAt)
	if !payloadBytes.Valid || payloadBytes.Int64 < 0 {
		return AdminListItem{}, ErrPayloadMismatch
	}
	item.PayloadBytes = payloadBytes.Int64
	switch item.PayloadKind {
	case domain.PayloadText:
		if filename.Valid || mediaType.Valid {
			return AdminListItem{}, ErrPayloadMismatch
		}
	case domain.PayloadFile:
		if item.PrivacyMode != domain.PrivacyStandard || !filename.Valid || !mediaType.Valid {
			return AdminListItem{}, ErrPayloadMismatch
		}
		item.FileFilename = filename.String
		item.FileMediaType = mediaType.String
	default:
		return AdminListItem{}, ErrPayloadMismatch
	}
	return item, nil
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

package share

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/domain"
)

const MaxTextBytes = 1 << 20

var (
	ErrNotFound     = errors.New("share not found")
	ErrExpired      = errors.New("share expired")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalidText  = errors.New("text content must be between 1 byte and 1 MiB")
	ErrExpiration   = errors.New("expiration must be in the future")
	ErrEmptyPatch   = errors.New("patch must change text or expiration")
)

type Text struct {
	Format  domain.TextFormat
	Content string
}

type Share struct {
	ID          capability.ShareID
	PayloadKind domain.PayloadKind
	PrivacyMode domain.PrivacyMode
	Text        Text
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ExpiresAt   *time.Time
}

type CreateInput struct {
	Text      Text
	ExpiresAt *time.Time
}

type Patch struct {
	Text          *Text
	ExpirationSet bool
	ExpiresAt     *time.Time
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func New(db *sql.DB, now func() time.Time) *Service {
	return &Service{db: db, now: now}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Share, capability.OwnerToken, error) {
	now := s.serverNow()
	if err := validateText(input.Text); err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	expiresAt, err := normalizeExpiration(input.ExpiresAt, now)
	if err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	id, err := capability.GenerateShareID()
	if err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	token, err := capability.GenerateOwnerToken()
	if err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	created := Share{
		ID:          id,
		PayloadKind: domain.PayloadText,
		PrivacyMode: domain.PrivacyStandard,
		Text:        input.Text,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   expiresAt,
	}
	if err := s.insert(ctx, created, token.Verifier()); err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	return created, token, nil
}

func (s *Service) Get(ctx context.Context, id capability.ShareID) (Share, error) {
	record, err := loadActive(ctx, s.db, id, s.serverNow())
	return record.Share, err
}

func (s *Service) AuthorizeOwner(ctx context.Context, id capability.ShareID, candidate string) error {
	record, err := loadActive(ctx, s.db, id, s.serverNow())
	if err != nil {
		return err
	}
	if !capability.VerifyOwnerToken(candidate, record.verifier) {
		return ErrUnauthorized
	}
	return nil
}

func (s *Service) Update(ctx context.Context, id capability.ShareID, candidate string, patch Patch) (Share, error) {
	now := s.serverNow()
	if patch.Text == nil && !patch.ExpirationSet {
		return Share{}, ErrEmptyPatch
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Share{}, fmt.Errorf("begin Share update: %w", err)
	}
	defer tx.Rollback()
	record, err := loadActive(ctx, tx, id, now)
	if err != nil {
		return Share{}, err
	}
	if !capability.VerifyOwnerToken(candidate, record.verifier) {
		return Share{}, ErrUnauthorized
	}
	if patch.Text != nil {
		if err := validateText(*patch.Text); err != nil {
			return Share{}, err
		}
		result, err := tx.ExecContext(ctx, "UPDATE standard_text_payloads SET format = ?, content = ? WHERE share_id = ?", patch.Text.Format, patch.Text.Content, id.String())
		if err != nil {
			return Share{}, fmt.Errorf("update text payload: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return Share{}, fmt.Errorf("count updated text payload rows: %w", err)
		}
		if rows != 1 {
			return Share{}, fmt.Errorf("update text payload affected %d rows", rows)
		}
	}
	var expiresAt *time.Time
	var result sql.Result
	if patch.ExpirationSet {
		expiresAt, err = normalizeExpiration(patch.ExpiresAt, now)
		if err != nil {
			return Share{}, err
		}
		result, err = tx.ExecContext(ctx, "UPDATE shares SET updated_at = ?, expires_at = ? WHERE id = ?", now.UnixMilli(), nullableMillis(expiresAt), id.String())
	} else {
		result, err = tx.ExecContext(ctx, "UPDATE shares SET updated_at = ? WHERE id = ?", now.UnixMilli(), id.String())
	}
	if err != nil {
		return Share{}, fmt.Errorf("update Share metadata: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Share{}, fmt.Errorf("count updated Share metadata rows: %w", err)
	}
	if rows != 1 {
		return Share{}, fmt.Errorf("update Share metadata affected %d rows", rows)
	}
	updated, err := loadActive(ctx, tx, id, now)
	if err != nil {
		return Share{}, err
	}
	if err := tx.Commit(); err != nil {
		return Share{}, fmt.Errorf("commit Share update: %w", err)
	}
	return updated.Share, nil
}

func (s *Service) Delete(ctx context.Context, id capability.ShareID, candidate string) error {
	now := s.serverNow()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Share delete: %w", err)
	}
	defer tx.Rollback()
	record, err := loadActive(ctx, tx, id, now)
	if err != nil {
		return err
	}
	if !capability.VerifyOwnerToken(candidate, record.verifier) {
		return ErrUnauthorized
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM shares WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("delete Share: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted Share rows: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("delete Share affected %d rows", rows)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Share delete: %w", err)
	}
	return nil
}

func (s *Service) insert(ctx context.Context, value Share, verifier capability.Verifier) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Share creation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO shares (id, payload_kind, privacy_mode, owner_token_verifier, created_at, updated_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, value.ID.String(), value.PayloadKind, value.PrivacyMode, verifier[:], value.CreatedAt.UnixMilli(), value.UpdatedAt.UnixMilli(), nullableMillis(value.ExpiresAt)); err != nil {
		return fmt.Errorf("insert Share metadata: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, ?, ?)", value.ID.String(), value.Text.Format, value.Text.Content); err != nil {
		return fmt.Errorf("insert text payload: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Share creation: %w", err)
	}
	return nil
}

func (s *Service) serverNow() time.Time {
	return s.now().UTC().Truncate(time.Millisecond)
}

func validateText(text Text) error {
	if _, err := domain.ParseTextFormat(string(text.Format)); err != nil {
		return err
	}
	if len(text.Content) < 1 || len(text.Content) > MaxTextBytes {
		return ErrInvalidText
	}
	return nil
}

func normalizeExpiration(value *time.Time, now time.Time) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	normalized := value.UTC().Truncate(time.Millisecond)
	if !normalized.After(now) {
		return nil, ErrExpiration
	}
	return &normalized, nil
}

func nullableMillis(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UnixMilli()
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type record struct {
	Share
	verifier capability.Verifier
}

func loadActive(ctx context.Context, db queryRower, id capability.ShareID, now time.Time) (record, error) {
	var value record
	var idValue, kind, privacy, format string
	var verifier []byte
	var createdAt, updatedAt int64
	var expiresAt sql.NullInt64
	err := db.QueryRowContext(ctx, `
		SELECT s.id, s.payload_kind, s.privacy_mode, s.owner_token_verifier,
		       s.created_at, s.updated_at, s.expires_at, p.format, p.content
		FROM shares s
		JOIN standard_text_payloads p ON p.share_id = s.id
		WHERE s.id = ? AND s.payload_kind = 'TEXT' AND s.privacy_mode = 'STANDARD'
	`, id.String()).Scan(&idValue, &kind, &privacy, &verifier, &createdAt, &updatedAt, &expiresAt, &format, &value.Text.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return record{}, ErrNotFound
	}
	if err != nil {
		return record{}, fmt.Errorf("load Share: %w", err)
	}
	parsedID, err := capability.ParseShareID(idValue)
	if err != nil {
		return record{}, fmt.Errorf("load Share ID: %w", err)
	}
	payloadKind, err := domain.ParsePayloadKind(kind)
	if err != nil {
		return record{}, fmt.Errorf("load payload kind: %w", err)
	}
	privacyMode, err := domain.ParsePrivacyMode(privacy)
	if err != nil {
		return record{}, fmt.Errorf("load privacy mode: %w", err)
	}
	textFormat, err := domain.ParseTextFormat(format)
	if err != nil {
		return record{}, fmt.Errorf("load text format: %w", err)
	}
	if len(verifier) != len(value.verifier) {
		return record{}, errors.New("load owner verifier: invalid length")
	}
	copy(value.verifier[:], verifier)
	value.ID = parsedID
	value.PayloadKind = payloadKind
	value.PrivacyMode = privacyMode
	value.Text.Format = textFormat
	value.CreatedAt = time.UnixMilli(createdAt).UTC()
	value.UpdatedAt = time.UnixMilli(updatedAt).UTC()
	if expiresAt.Valid {
		expires := time.UnixMilli(expiresAt.Int64).UTC()
		value.ExpiresAt = &expires
	}
	if domain.IsExpired(value.ExpiresAt, now) {
		return record{}, ErrExpired
	}
	return value, nil
}

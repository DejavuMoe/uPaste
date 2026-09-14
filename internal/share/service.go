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

const (
	MaxTextBytes            = 1 << 20
	EncryptedTextProtocolV1 = "UPASTE_AES_GCM_V1"
	EncryptedNonceBytes     = 12
	MinEncryptedCipherBytes = 19
	MaxEncryptedCipherBytes = MaxTextBytes + 2 + 16
)

var (
	ErrNotFound             = errors.New("share not found")
	ErrExpired              = errors.New("share expired")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrInvalidText          = errors.New("text content must be between 1 byte and 1 MiB")
	ErrInvalidEncryptedText = errors.New("encrypted text payload is invalid")
	ErrPayloadMismatch      = errors.New("payload does not match Share privacy mode")
	ErrExpiration           = errors.New("expiration must be in the future")
	ErrEmptyPatch           = errors.New("patch must change text or expiration")
)

type Text struct {
	Format  domain.TextFormat
	Content string
}

type EncryptedText struct {
	Protocol   string
	Nonce      []byte
	Ciphertext []byte
}

type Share struct {
	ID            capability.ShareID
	PayloadKind   domain.PayloadKind
	PrivacyMode   domain.PrivacyMode
	Text          *Text
	EncryptedText *EncryptedText
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     *time.Time
}

type CreateInput struct {
	Text      Text
	ExpiresAt *time.Time
}

type EncryptedCreateInput struct {
	EncryptedText EncryptedText
	ExpiresAt     *time.Time
}

type Patch struct {
	Text          *Text
	EncryptedText *EncryptedText
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
	if err := validateText(input.Text); err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	return s.create(ctx, domain.PrivacyStandard, &input.Text, nil, input.ExpiresAt)
}

func (s *Service) CreateEncrypted(ctx context.Context, input EncryptedCreateInput) (Share, capability.OwnerToken, error) {
	if err := validateEncryptedText(input.EncryptedText); err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	return s.create(ctx, domain.PrivacyEncrypted, nil, &input.EncryptedText, input.ExpiresAt)
}

func (s *Service) create(ctx context.Context, privacy domain.PrivacyMode, text *Text, encrypted *EncryptedText, expiration *time.Time) (Share, capability.OwnerToken, error) {
	now := s.serverNow()
	expiresAt, err := normalizeExpiration(expiration, now)
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
		ID: id, PayloadKind: domain.PayloadText, PrivacyMode: privacy,
		Text: text, EncryptedText: encrypted,
		CreatedAt: now, UpdatedAt: now, ExpiresAt: expiresAt,
	}
	if err := s.insert(ctx, created, token.Verifier()); err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	return created, token, nil
}

func (s *Service) Get(ctx context.Context, id capability.ShareID) (Share, error) {
	return loadActive(ctx, s.db, id, s.serverNow())
}

func (s *Service) AuthorizeOwner(ctx context.Context, id capability.ShareID, candidate string) (domain.PrivacyMode, error) {
	record, err := loadAuthorization(ctx, s.db, id, s.serverNow())
	if err != nil {
		return "", err
	}
	if !capability.VerifyOwnerToken(candidate, record.verifier) {
		return "", ErrUnauthorized
	}
	return record.privacy, nil
}

func (s *Service) Update(ctx context.Context, id capability.ShareID, candidate string, patch Patch) (Share, error) {
	now := s.serverNow()
	if patch.Text == nil && patch.EncryptedText == nil && !patch.ExpirationSet {
		return Share{}, ErrEmptyPatch
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Share{}, fmt.Errorf("begin Share update: %w", err)
	}
	defer tx.Rollback()
	authorization, err := loadAuthorization(ctx, tx, id, now)
	if err != nil {
		return Share{}, err
	}
	if !capability.VerifyOwnerToken(candidate, authorization.verifier) {
		return Share{}, ErrUnauthorized
	}
	switch authorization.privacy {
	case domain.PrivacyStandard:
		if patch.EncryptedText != nil {
			return Share{}, ErrPayloadMismatch
		}
		if patch.Text != nil {
			if err := validateText(*patch.Text); err != nil {
				return Share{}, err
			}
			if err := updateStandardText(ctx, tx, id, *patch.Text); err != nil {
				return Share{}, err
			}
		}
	case domain.PrivacyEncrypted:
		if patch.Text != nil {
			return Share{}, ErrPayloadMismatch
		}
		if patch.EncryptedText != nil {
			if err := validateEncryptedText(*patch.EncryptedText); err != nil {
				return Share{}, err
			}
			if err := updateEncryptedText(ctx, tx, id, *patch.EncryptedText); err != nil {
				return Share{}, err
			}
		}
	default:
		return Share{}, ErrPayloadMismatch
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
	if err := requireOneRow(result, "update Share metadata"); err != nil {
		return Share{}, err
	}
	updated, err := loadActive(ctx, tx, id, now)
	if err != nil {
		return Share{}, err
	}
	if err := tx.Commit(); err != nil {
		return Share{}, fmt.Errorf("commit Share update: %w", err)
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id capability.ShareID, candidate string) error {
	now := s.serverNow()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Share delete: %w", err)
	}
	defer tx.Rollback()
	authorization, err := loadAuthorization(ctx, tx, id, now)
	if err != nil {
		return err
	}
	if !capability.VerifyOwnerToken(candidate, authorization.verifier) {
		return ErrUnauthorized
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM shares WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("delete Share: %w", err)
	}
	if err := requireOneRow(result, "delete Share"); err != nil {
		return err
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
	switch value.PrivacyMode {
	case domain.PrivacyStandard:
		if value.Text == nil || value.EncryptedText != nil {
			return ErrPayloadMismatch
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO standard_text_payloads (share_id, format, content) VALUES (?, ?, ?)", value.ID.String(), value.Text.Format, value.Text.Content); err != nil {
			return fmt.Errorf("insert text payload: %w", err)
		}
	case domain.PrivacyEncrypted:
		if value.EncryptedText == nil || value.Text != nil {
			return ErrPayloadMismatch
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO encrypted_text_payloads (share_id, protocol, nonce, ciphertext) VALUES (?, ?, ?, ?)", value.ID.String(), value.EncryptedText.Protocol, value.EncryptedText.Nonce, value.EncryptedText.Ciphertext); err != nil {
			return fmt.Errorf("insert encrypted text payload: %w", err)
		}
	default:
		return ErrPayloadMismatch
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Share creation: %w", err)
	}
	return nil
}

func updateStandardText(ctx context.Context, tx *sql.Tx, id capability.ShareID, text Text) error {
	result, err := tx.ExecContext(ctx, "UPDATE standard_text_payloads SET format = ?, content = ? WHERE share_id = ?", text.Format, text.Content, id.String())
	if err != nil {
		return fmt.Errorf("update text payload: %w", err)
	}
	return requireOneRow(result, "update text payload")
}

func updateEncryptedText(ctx context.Context, tx *sql.Tx, id capability.ShareID, encrypted EncryptedText) error {
	result, err := tx.ExecContext(ctx, "UPDATE encrypted_text_payloads SET protocol = ?, nonce = ?, ciphertext = ? WHERE share_id = ?", encrypted.Protocol, encrypted.Nonce, encrypted.Ciphertext, id.String())
	if err != nil {
		return fmt.Errorf("update encrypted text payload: %w", err)
	}
	return requireOneRow(result, "update encrypted text payload")
}

func requireOneRow(result sql.Result, operation string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count %s rows: %w", operation, err)
	}
	if rows != 1 {
		return fmt.Errorf("%s affected %d rows", operation, rows)
	}
	return nil
}

func (s *Service) serverNow() time.Time { return s.now().UTC().Truncate(time.Millisecond) }

func validateText(text Text) error {
	if _, err := domain.ParseTextFormat(string(text.Format)); err != nil {
		return err
	}
	if len(text.Content) < 1 || len(text.Content) > MaxTextBytes {
		return ErrInvalidText
	}
	return nil
}

func validateEncryptedText(value EncryptedText) error {
	if value.Protocol != EncryptedTextProtocolV1 || len(value.Nonce) != EncryptedNonceBytes || len(value.Ciphertext) < MinEncryptedCipherBytes || len(value.Ciphertext) > MaxEncryptedCipherBytes {
		return ErrInvalidEncryptedText
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

type authorizationRecord struct {
	privacy  domain.PrivacyMode
	verifier capability.Verifier
}

func loadAuthorization(ctx context.Context, db queryRower, id capability.ShareID, now time.Time) (authorizationRecord, error) {
	var record authorizationRecord
	var kind, privacy string
	var verifier []byte
	var expiresAt sql.NullInt64
	err := db.QueryRowContext(ctx, `
		SELECT payload_kind, privacy_mode, owner_token_verifier, expires_at
		FROM shares WHERE id = ?
	`, id.String()).Scan(&kind, &privacy, &verifier, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return authorizationRecord{}, ErrNotFound
	}
	if err != nil {
		return authorizationRecord{}, fmt.Errorf("load Share authorization: %w", err)
	}
	payloadKind, err := domain.ParsePayloadKind(kind)
	if err != nil || payloadKind != domain.PayloadText {
		return authorizationRecord{}, ErrNotFound
	}
	record.privacy, err = domain.ParsePrivacyMode(privacy)
	if err != nil {
		return authorizationRecord{}, fmt.Errorf("load privacy mode: %w", err)
	}
	if len(verifier) != len(record.verifier) {
		return authorizationRecord{}, errors.New("load owner verifier: invalid length")
	}
	copy(record.verifier[:], verifier)
	if isExpired(expiresAt, now) {
		return authorizationRecord{}, ErrExpired
	}
	return record, nil
}

func loadActive(ctx context.Context, db queryRower, id capability.ShareID, now time.Time) (Share, error) {
	var value Share
	var idValue, kind, privacy string
	var createdAt, updatedAt int64
	var expiresAt sql.NullInt64
	var format, content, protocol sql.NullString
	var nonce, ciphertext []byte
	err := db.QueryRowContext(ctx, `
		SELECT s.id, s.payload_kind, s.privacy_mode, s.created_at, s.updated_at, s.expires_at,
		       p.format, p.content, e.protocol, e.nonce, e.ciphertext
		FROM shares s
		LEFT JOIN standard_text_payloads p ON p.share_id = s.id
		LEFT JOIN encrypted_text_payloads e ON e.share_id = s.id
		WHERE s.id = ? AND s.payload_kind = 'TEXT'
	`, id.String()).Scan(&idValue, &kind, &privacy, &createdAt, &updatedAt, &expiresAt, &format, &content, &protocol, &nonce, &ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	if err != nil {
		return Share{}, fmt.Errorf("load Share: %w", err)
	}
	if isExpired(expiresAt, now) {
		return Share{}, ErrExpired
	}
	parsedID, err := capability.ParseShareID(idValue)
	if err != nil {
		return Share{}, fmt.Errorf("load Share ID: %w", err)
	}
	value.ID = parsedID
	value.PayloadKind, err = domain.ParsePayloadKind(kind)
	if err != nil {
		return Share{}, fmt.Errorf("load payload kind: %w", err)
	}
	value.PrivacyMode, err = domain.ParsePrivacyMode(privacy)
	if err != nil {
		return Share{}, fmt.Errorf("load privacy mode: %w", err)
	}
	value.CreatedAt = time.UnixMilli(createdAt).UTC()
	value.UpdatedAt = time.UnixMilli(updatedAt).UTC()
	value.ExpiresAt = expirationTime(expiresAt)
	switch value.PrivacyMode {
	case domain.PrivacyStandard:
		if !format.Valid || !content.Valid || protocol.Valid || nonce != nil || ciphertext != nil {
			return Share{}, ErrPayloadMismatch
		}
		textFormat, err := domain.ParseTextFormat(format.String)
		if err != nil {
			return Share{}, fmt.Errorf("load text format: %w", err)
		}
		value.Text = &Text{Format: textFormat, Content: content.String}
	case domain.PrivacyEncrypted:
		if format.Valid || content.Valid || !protocol.Valid {
			return Share{}, ErrPayloadMismatch
		}
		value.EncryptedText = &EncryptedText{Protocol: protocol.String, Nonce: nonce, Ciphertext: ciphertext}
		if err := validateEncryptedText(*value.EncryptedText); err != nil {
			return Share{}, fmt.Errorf("load encrypted text: %w", err)
		}
	default:
		return Share{}, ErrPayloadMismatch
	}
	return value, nil
}

func isExpired(value sql.NullInt64, now time.Time) bool {
	return domain.IsExpired(expirationTime(value), now)
}

func expirationTime(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	expiresAt := time.UnixMilli(value.Int64).UTC()
	return &expiresAt
}

package share

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/domain"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
)

const (
	MaxTextBytes            = 1 << 20
	EncryptedTextProtocolV1 = "UPASTE_AES_GCM_V1"
	EncryptedNonceBytes     = 12
	MinEncryptedCipherBytes = 19
	MaxEncryptedCipherBytes = MaxTextBytes + 2 + 16
	MaxFileBytes            = 64 << 20
	FileTransferDeadline    = 10 * time.Minute
)

var (
	ErrNotFound             = errors.New("share not found")
	ErrExpired              = errors.New("share expired")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrInvalidText          = errors.New("text content must be between 1 byte and 1 MiB")
	ErrInvalidEncryptedText = errors.New("encrypted text payload is invalid")
	ErrPayloadMismatch      = errors.New("payload does not match Share privacy mode")
	ErrExpiration           = errors.New("expiration must be in the future")
	ErrRetention            = errors.New("expiration exceeds the configured public retention limit")
	ErrEmptyPatch           = errors.New("patch must change text or expiration")
)

// RetentionPolicy is the authoritative public retention bound.
type RetentionPolicy struct {
	Public     bool
	DefaultTTL time.Duration
	MaxTTL     time.Duration
}

type CleanupError struct {
	StorageKey string
	Err        error
}

func (err *CleanupError) Error() string { return "remove deleted File object: " + err.Err.Error() }
func (err *CleanupError) Unwrap() error { return err.Err }

type Text struct {
	Format  domain.TextFormat
	Content string
}

type EncryptedText struct {
	Protocol   string
	Nonce      []byte
	Ciphertext []byte
}

type File struct {
	StorageKey string
	Filename   string
	Size       int64
	MediaType  string
	SHA256     [sha256.Size]byte
}

type Share struct {
	ID            capability.ShareID
	PayloadKind   domain.PayloadKind
	PrivacyMode   domain.PrivacyMode
	Text          *Text
	EncryptedText *EncryptedText
	File          *File
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
	db        *sql.DB
	store     objectstore.Store
	now       func() time.Time
	retention RetentionPolicy
}

// SetRetentionPolicy installs the authoritative public retention policy.
func (service *Service) SetRetentionPolicy(policy RetentionPolicy) {
	service.retention = policy
}

func New(db *sql.DB, now func() time.Time) *Service {
	return &Service{db: db, now: now}
}

func NewWithStore(db *sql.DB, store objectstore.Store, now func() time.Time) *Service {
	return &Service{db: db, store: store, now: now}
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

func (s *Service) StageFile(ctx context.Context, source io.Reader) (objectstore.Staged, error) {
	if s.store == nil {
		return nil, errors.New("object storage is unavailable")
	}
	return s.store.Stage(ctx, source, MaxFileBytes)
}

func (s *Service) CreateFile(ctx context.Context, filename string, staged objectstore.Staged, expiration *time.Time) (value Share, token capability.OwnerToken, err error) {
	if s.store == nil || staged == nil {
		return Share{}, capability.OwnerToken{}, errors.New("object storage is unavailable")
	}
	defer staged.Abort()
	now := s.serverNow()
	expiresAt, err := s.normalizeExpiration(expiration, now)
	if err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	id, err := capability.GenerateShareID()
	if err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	token, err = capability.GenerateOwnerToken()
	if err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	file := &File{
		StorageKey: staged.Key(), Filename: filename, Size: staged.Size(),
		MediaType: http.DetectContentType(staged.Sniff()), SHA256: staged.SHA256(),
	}
	committed := false
	defer func() {
		if !committed {
			if cleanupErr := s.store.Delete(file.StorageKey); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("remove uncommitted File object: %w", cleanupErr))
			}
		}
	}()
	created := Share{
		ID: id, PayloadKind: domain.PayloadFile, PrivacyMode: domain.PrivacyStandard,
		File: file, CreatedAt: now, UpdatedAt: now, ExpiresAt: expiresAt,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Share{}, capability.OwnerToken{}, fmt.Errorf("begin File Share creation: %w", err)
	}
	defer tx.Rollback()
	if err := insertMetadata(ctx, tx, created, token.Verifier()); err != nil {
		return Share{}, capability.OwnerToken{}, err
	}
	if err := staged.Commit(); err != nil {
		return Share{}, capability.OwnerToken{}, fmt.Errorf("finalize File object: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO file_payloads (share_id, storage_key, original_filename, size_bytes, detected_media_type, content_sha256)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id.String(), file.StorageKey, file.Filename, file.Size, file.MediaType, file.SHA256[:]); err != nil {
		return Share{}, capability.OwnerToken{}, fmt.Errorf("insert File payload: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Share{}, capability.OwnerToken{}, fmt.Errorf("commit File Share creation: %w", err)
	}
	committed = true
	return created, token, nil
}

func (s *Service) OpenFile(ctx context.Context, id capability.ShareID) (Share, objectstore.ReadSeekCloser, error) {
	value, err := s.Get(ctx, id)
	if err != nil {
		return Share{}, nil, err
	}
	if value.File == nil || s.store == nil {
		return Share{}, nil, ErrNotFound
	}
	object, err := s.store.Open(value.File.StorageKey)
	if err != nil {
		return Share{}, nil, fmt.Errorf("open File object: %w", err)
	}
	return value, object, nil
}

func (s *Service) create(ctx context.Context, privacy domain.PrivacyMode, text *Text, encrypted *EncryptedText, expiration *time.Time) (Share, capability.OwnerToken, error) {
	now := s.serverNow()
	expiresAt, err := s.normalizeExpiration(expiration, now)
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

func (s *Service) AuthorizeOwner(ctx context.Context, id capability.ShareID, candidate string) (domain.PayloadKind, domain.PrivacyMode, error) {
	record, err := loadAuthorization(ctx, s.db, id, s.serverNow())
	if err != nil {
		return "", "", err
	}
	if !capability.VerifyOwnerToken(candidate, record.verifier) {
		return "", "", ErrUnauthorized
	}
	return record.kind, record.privacy, nil
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
	switch {
	case authorization.kind == domain.PayloadFile:
		if patch.Text != nil || patch.EncryptedText != nil {
			return Share{}, ErrPayloadMismatch
		}
	case authorization.privacy == domain.PrivacyStandard:
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
	case authorization.privacy == domain.PrivacyEncrypted:
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
		expiresAt, err = s.validateUpdatedExpiration(patch.ExpiresAt, authorization.createdAt, now)
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
	var storageKey string
	if authorization.kind == domain.PayloadFile {
		if err := tx.QueryRowContext(ctx, "SELECT storage_key FROM file_payloads WHERE share_id = ?", id.String()).Scan(&storageKey); err != nil {
			return fmt.Errorf("load File storage key: %w", err)
		}
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
	if storageKey != "" {
		if s.store == nil {
			return &CleanupError{StorageKey: storageKey, Err: errors.New("object storage is unavailable")}
		}
		if err := s.store.Delete(storageKey); err != nil {
			return &CleanupError{StorageKey: storageKey, Err: err}
		}
	}
	return nil
}

func (s *Service) insert(ctx context.Context, value Share, verifier capability.Verifier) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Share creation: %w", err)
	}
	defer tx.Rollback()
	if err := insertMetadata(ctx, tx, value, verifier); err != nil {
		return err
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

func insertMetadata(ctx context.Context, tx *sql.Tx, value Share, verifier capability.Verifier) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO shares (id, payload_kind, privacy_mode, owner_token_verifier, created_at, updated_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, value.ID.String(), value.PayloadKind, value.PrivacyMode, verifier[:], value.CreatedAt.UnixMilli(), value.UpdatedAt.UnixMilli(), nullableMillis(value.ExpiresAt)); err != nil {
		return fmt.Errorf("insert Share metadata: %w", err)
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

func (service *Service) normalizeExpiration(value *time.Time, now time.Time) (*time.Time, error) {
	if service.retention.Public {
		if value == nil {
			expiresAt := now.Add(service.retention.DefaultTTL)
			return &expiresAt, nil
		}
		return service.validateUpdatedExpiration(value, now, now)
	}
	if value == nil {
		return nil, nil
	}
	normalized := value.UTC().Truncate(time.Millisecond)
	if !normalized.After(now) {
		return nil, ErrExpiration
	}
	return &normalized, nil
}

func (service *Service) validateUpdatedExpiration(value *time.Time, createdAt, now time.Time) (*time.Time, error) {
	if value == nil {
		if service.retention.Public {
			return nil, ErrRetention
		}
		return nil, nil
	}
	normalized := value.UTC().Truncate(time.Millisecond)
	if !normalized.After(now) {
		return nil, ErrExpiration
	}
	if service.retention.Public {
		horizon := createdAt.Add(service.retention.MaxTTL)
		if normalized.After(horizon) {
			return nil, ErrRetention
		}
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
	kind      domain.PayloadKind
	privacy   domain.PrivacyMode
	verifier  capability.Verifier
	createdAt time.Time
}

func loadAuthorization(ctx context.Context, db queryRower, id capability.ShareID, now time.Time) (authorizationRecord, error) {
	var record authorizationRecord
	var kind, privacy string
	var verifier []byte
	var expiresAt sql.NullInt64
	var createdAt int64
	err := db.QueryRowContext(ctx, `
		SELECT payload_kind, privacy_mode, owner_token_verifier, created_at, expires_at
		FROM shares WHERE id = ?
	`, id.String()).Scan(&kind, &privacy, &verifier, &createdAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return authorizationRecord{}, ErrNotFound
	}
	if err != nil {
		return authorizationRecord{}, fmt.Errorf("load Share authorization: %w", err)
	}
	record.kind, err = domain.ParsePayloadKind(kind)
	if err != nil {
		return authorizationRecord{}, fmt.Errorf("load payload kind: %w", err)
	}
	record.privacy, err = domain.ParsePrivacyMode(privacy)
	if err != nil {
		return authorizationRecord{}, fmt.Errorf("load privacy mode: %w", err)
	}
	if len(verifier) != len(record.verifier) {
		return authorizationRecord{}, errors.New("load owner verifier: invalid length")
	}
	copy(record.verifier[:], verifier)
	record.createdAt = time.UnixMilli(createdAt).UTC()
	if isExpired(expiresAt, now) {
		return authorizationRecord{}, ErrExpired
	}
	return record, nil
}

func loadActive(ctx context.Context, db queryRower, id capability.ShareID, now time.Time) (Share, error) {
	var value Share
	var idValue, kind, privacy string
	var createdAt, updatedAt int64
	var expiresAt, fileSize sql.NullInt64
	var format, content, protocol, storageKey, filename, mediaType sql.NullString
	var nonce, ciphertext, fileHash []byte
	err := db.QueryRowContext(ctx, `
		SELECT s.id, s.payload_kind, s.privacy_mode, s.created_at, s.updated_at, s.expires_at,
		       p.format, p.content, e.protocol, e.nonce, e.ciphertext,
		       f.storage_key, f.original_filename, f.size_bytes, f.detected_media_type, f.content_sha256
		FROM shares s
		LEFT JOIN standard_text_payloads p ON p.share_id = s.id
		LEFT JOIN encrypted_text_payloads e ON e.share_id = s.id
		LEFT JOIN file_payloads f ON f.share_id = s.id
		WHERE s.id = ?
	`, id.String()).Scan(
		&idValue, &kind, &privacy, &createdAt, &updatedAt, &expiresAt,
		&format, &content, &protocol, &nonce, &ciphertext,
		&storageKey, &filename, &fileSize, &mediaType, &fileHash,
	)
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
	switch value.PayloadKind {
	case domain.PayloadText:
		if storageKey.Valid || filename.Valid || fileSize.Valid || mediaType.Valid || fileHash != nil {
			return Share{}, ErrPayloadMismatch
		}
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
	case domain.PayloadFile:
		if value.PrivacyMode != domain.PrivacyStandard || format.Valid || content.Valid || protocol.Valid || nonce != nil || ciphertext != nil || !storageKey.Valid || !filename.Valid || !fileSize.Valid || !mediaType.Valid || len(fileHash) != sha256.Size {
			return Share{}, ErrPayloadMismatch
		}
		value.File = &File{StorageKey: storageKey.String, Filename: filename.String, Size: fileSize.Int64, MediaType: mediaType.String}
		copy(value.File.SHA256[:], fileHash)
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

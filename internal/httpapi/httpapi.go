package httpapi

import (
	"encoding/base64"
	jsonv1 "encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/domain"
	"github.com/DejavuMoe/uPaste/internal/share"
)

const maxJSONBytes = 2 << 20

var (
	errBodyTooLarge       = errors.New("request body is too large")
	errCiphertextTooLarge = errors.New("encrypted ciphertext is too large")
)

type API struct {
	shares *share.Service
	log    *slog.Logger
}

func New(shares *share.Service, log *slog.Logger) http.Handler {
	api := &API{shares: shares, log: log}
	return securityHeaders(http.HandlerFunc(api.route))
}

func (api *API) route(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/v1/shares":
		api.collection(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/shares/"):
		api.item(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/"):
		api.apiNotFound(w, r)
	case r.URL.Path == "/raw" || strings.HasPrefix(r.URL.Path, "/raw/"):
		api.raw(w, r)
	default:
		http.NotFound(w, r)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
		}
		if r.URL.Path == "/raw" || strings.HasPrefix(r.URL.Path, "/raw/") {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; sandbox")
		}
		next.ServeHTTP(w, r)
	})
}

func (api *API) collection(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/shares" {
		api.writeError(w, http.StatusNotFound, "not_found", "share not found")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		api.writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
		return
	}
	api.create(w, r)
}

func (api *API) item(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "GET, PATCH, DELETE")
		api.writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
		return
	}
	idValue := strings.TrimPrefix(r.URL.Path, "/api/v1/shares/")
	if idValue == "" || strings.Contains(idValue, "/") {
		api.writeError(w, http.StatusNotFound, "not_found", "share not found")
		return
	}
	id, err := capability.ParseShareID(idValue)
	if err != nil {
		api.writeError(w, http.StatusNotFound, "not_found", "share not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		api.read(w, r, id)
	case http.MethodPatch:
		api.patch(w, r, id)
	case http.MethodDelete:
		api.delete(w, r, id)
	}
}

func (api *API) raw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idValue := strings.TrimPrefix(r.URL.Path, "/raw/")
	if idValue == "" || strings.Contains(idValue, "/") {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}
	id, err := capability.ParseShareID(idValue)
	if err != nil {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}
	value, err := api.shares.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, share.ErrExpired) {
			http.Error(w, "share expired", http.StatusGone)
		} else if errors.Is(err, share.ErrNotFound) {
			http.Error(w, "share not found", http.StatusNotFound)
		} else {
			api.log.Error("request failed", "operation", "raw Share", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	if value.EncryptedText != nil {
		http.Error(w, "raw view unavailable for encrypted share", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, value.Text.Content)
}

func (api *API) apiNotFound(w http.ResponseWriter, _ *http.Request) {
	api.writeError(w, http.StatusNotFound, "not_found", "share not found")
}

type textRequest struct {
	Format  *string `json:"format"`
	Content *string `json:"content"`
}

type encryptedTextRequest struct {
	Protocol   *string `json:"protocol"`
	Nonce      *string `json:"nonce"`
	Ciphertext *string `json:"ciphertext"`
}

type nullableTime struct {
	set   bool
	value *time.Time
}

func (value *nullableTime) UnmarshalJSON(data []byte) error {
	value.set = true
	if string(data) == "null" {
		value.value = nil
		return nil
	}
	var encoded string
	if err := jsonv2.Unmarshal(data, &encoded); err != nil {
		return errors.New("timestamp must be an RFC3339 string or null")
	}
	parsed, err := time.Parse(time.RFC3339Nano, encoded)
	if err != nil {
		return errors.New("timestamp must use RFC3339")
	}
	value.value = &parsed
	return nil
}

type optionalText struct {
	set   bool
	value *textRequest
}

func (value *optionalText) UnmarshalJSON(data []byte) error {
	value.set = true
	if string(data) == "null" {
		return nil
	}
	var text textRequest
	if err := jsonv2.Unmarshal(data, &text, jsonv2.RejectUnknownMembers(true)); err != nil {
		return err
	}
	value.value = &text
	return nil
}

type optionalEncryptedText struct {
	set   bool
	value *encryptedTextRequest
}

func (value *optionalEncryptedText) UnmarshalJSON(data []byte) error {
	value.set = true
	if string(data) == "null" {
		return nil
	}
	var encrypted encryptedTextRequest
	if err := jsonv2.Unmarshal(data, &encrypted, jsonv2.RejectUnknownMembers(true)); err != nil {
		return err
	}
	value.value = &encrypted
	return nil
}

type createRequest struct {
	PayloadKind   *string               `json:"payload_kind"`
	PrivacyMode   *string               `json:"privacy_mode"`
	Text          optionalText          `json:"text"`
	EncryptedText optionalEncryptedText `json:"encrypted_text"`
	ExpiresAt     nullableTime          `json:"expires_at"`
}

type patchRequest struct {
	Text          optionalText          `json:"text"`
	EncryptedText optionalEncryptedText `json:"encrypted_text"`
	ExpiresAt     nullableTime          `json:"expires_at"`
}

func (api *API) create(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r, api.writeError) {
		return
	}
	var request createRequest
	if err := decodeJSON(w, r, &request); err != nil {
		api.decodeError(w, err)
		return
	}
	if request.PayloadKind == nil || request.PrivacyMode == nil || !request.ExpiresAt.set {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "required field is missing")
		return
	}
	kind, err := domain.ParsePayloadKind(*request.PayloadKind)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload_kind")
		return
	}
	privacy, err := domain.ParsePrivacyMode(*request.PrivacyMode)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "invalid privacy_mode")
		return
	}
	if kind != domain.PayloadText {
		api.writeError(w, http.StatusUnprocessableEntity, "unsupported_share_type", "share type is not implemented")
		return
	}
	var value share.Share
	var token capability.OwnerToken
	switch privacy {
	case domain.PrivacyStandard:
		if !request.Text.set || request.Text.value == nil || request.EncryptedText.set {
			api.writeError(w, http.StatusBadRequest, "invalid_request", "text payload does not match privacy_mode")
			return
		}
		text, ok := api.validateText(w, request.Text.value)
		if !ok {
			return
		}
		value, token, err = api.shares.Create(r.Context(), share.CreateInput{Text: text, ExpiresAt: request.ExpiresAt.value})
	case domain.PrivacyEncrypted:
		if !request.EncryptedText.set || request.EncryptedText.value == nil || request.Text.set {
			api.writeError(w, http.StatusBadRequest, "invalid_request", "encrypted_text payload does not match privacy_mode")
			return
		}
		encrypted, ok := api.validateEncryptedText(w, request.EncryptedText.value)
		if !ok {
			return
		}
		value, token, err = api.shares.CreateEncrypted(r.Context(), share.EncryptedCreateInput{EncryptedText: encrypted, ExpiresAt: request.ExpiresAt.value})
	}
	if err != nil {
		api.serviceError(w, "create Share", err)
		return
	}
	w.Header().Set("Location", "/api/v1/shares/"+value.ID.String())
	api.writeJSON(w, http.StatusCreated, createResponse{Share: responseFromShare(value), OwnerToken: token.Reveal()})
}

func (api *API) read(w http.ResponseWriter, r *http.Request, id capability.ShareID) {
	value, err := api.shares.Get(r.Context(), id)
	if err != nil {
		api.serviceError(w, "read Share", err)
		return
	}
	api.writeJSON(w, http.StatusOK, shareResponse{Share: responseFromShare(value)})
}

func (api *API) patch(w http.ResponseWriter, r *http.Request, id capability.ShareID) {
	candidate, ok := api.authorization(w, r)
	if !ok {
		return
	}
	privacy, err := api.shares.AuthorizeOwner(r.Context(), id, candidate)
	if err != nil {
		api.serviceError(w, "authorize Share update", err)
		return
	}
	if !requireJSON(w, r, api.writeError) {
		return
	}
	var request patchRequest
	if err := decodeJSON(w, r, &request); err != nil {
		api.decodeError(w, err)
		return
	}
	if !request.Text.set && !request.EncryptedText.set && !request.ExpiresAt.set {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "patch must change text or expiration")
		return
	}
	patch := share.Patch{ExpirationSet: request.ExpiresAt.set, ExpiresAt: request.ExpiresAt.value}
	switch privacy {
	case domain.PrivacyStandard:
		if request.EncryptedText.set {
			api.writeError(w, http.StatusBadRequest, "invalid_request", "encrypted_text is not valid for this Share")
			return
		}
		if request.Text.set {
			if request.Text.value == nil {
				api.writeError(w, http.StatusBadRequest, "invalid_request", "text must be an object")
				return
			}
			text, ok := api.validateText(w, request.Text.value)
			if !ok {
				return
			}
			patch.Text = &text
		}
	case domain.PrivacyEncrypted:
		if request.Text.set {
			api.writeError(w, http.StatusBadRequest, "invalid_request", "text is not valid for this Share")
			return
		}
		if request.EncryptedText.set {
			if request.EncryptedText.value == nil {
				api.writeError(w, http.StatusBadRequest, "invalid_request", "encrypted_text must be an object")
				return
			}
			encrypted, ok := api.validateEncryptedText(w, request.EncryptedText.value)
			if !ok {
				return
			}
			patch.EncryptedText = &encrypted
		}
	}
	value, err := api.shares.Update(r.Context(), id, candidate, patch)
	if err != nil {
		api.serviceError(w, "update Share", err)
		return
	}
	api.writeJSON(w, http.StatusOK, shareResponse{Share: responseFromShare(value)})
}

func (api *API) delete(w http.ResponseWriter, r *http.Request, id capability.ShareID) {
	candidate, ok := api.authorization(w, r)
	if !ok {
		return
	}
	if err := api.shares.Delete(r.Context(), id, candidate); err != nil {
		api.serviceError(w, "delete Share", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) authorization(w http.ResponseWriter, r *http.Request) (string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		api.unauthorized(w)
		return "", false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		api.unauthorized(w)
		return "", false
	}
	return parts[1], true
}

func (api *API) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	api.writeError(w, http.StatusUnauthorized, "unauthorized", "owner authorization required")
}

func requireJSON(w http.ResponseWriter, r *http.Request, writeError func(http.ResponseWriter, int, string, string)) bool {
	for _, encoding := range r.Header.Values("Content-Encoding") {
		if !strings.EqualFold(strings.TrimSpace(encoding), "identity") {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "content encoding is not supported")
			return false
		}
	}
	values := r.Header.Values("Content-Type")
	if len(values) != 1 {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	mediaType, _, err := mime.ParseMediaType(values[0])
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if r.ContentLength > maxJSONBytes {
		return errBodyTooLarge
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBytes)
	if err := jsonv2.UnmarshalRead(r.Body, destination, jsonv2.RejectUnknownMembers(true)); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errBodyTooLarge
		}
		return err
	}
	return nil
}

func (api *API) validateText(w http.ResponseWriter, request *textRequest) (share.Text, bool) {
	if request.Format == nil || request.Content == nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "text format and content are required")
		return share.Text{}, false
	}
	format, err := domain.ParseTextFormat(*request.Format)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "invalid text format")
		return share.Text{}, false
	}
	if len(*request.Content) == 0 {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "text content must not be empty")
		return share.Text{}, false
	}
	if len(*request.Content) > share.MaxTextBytes {
		api.writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "text content is too large")
		return share.Text{}, false
	}
	return share.Text{Format: format, Content: *request.Content}, true
}

func (api *API) validateEncryptedText(w http.ResponseWriter, request *encryptedTextRequest) (share.EncryptedText, bool) {
	if request.Protocol == nil || request.Nonce == nil || request.Ciphertext == nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "encrypted_text fields are required")
		return share.EncryptedText{}, false
	}
	if *request.Protocol != share.EncryptedTextProtocolV1 {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "invalid encrypted_text protocol")
		return share.EncryptedText{}, false
	}
	nonce, err := decodeBase64URL(*request.Nonce, share.EncryptedNonceBytes, share.EncryptedNonceBytes)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "invalid encrypted_text nonce")
		return share.EncryptedText{}, false
	}
	ciphertext, err := decodeBase64URL(*request.Ciphertext, share.MinEncryptedCipherBytes, share.MaxEncryptedCipherBytes)
	if errors.Is(err, errCiphertextTooLarge) {
		api.writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "encrypted ciphertext is too large")
		return share.EncryptedText{}, false
	}
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "invalid encrypted_text ciphertext")
		return share.EncryptedText{}, false
	}
	return share.EncryptedText{Protocol: *request.Protocol, Nonce: nonce, Ciphertext: ciphertext}, true
}

func decodeBase64URL(value string, minimum, maximum int) ([]byte, error) {
	for _, character := range []byte(value) {
		if !((character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' || character == '_') {
			return nil, errors.New("invalid canonical base64url")
		}
	}
	if len(value) > base64.RawURLEncoding.EncodedLen(maximum) {
		return nil, errCiphertextTooLarge
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) < minimum || len(decoded) > maximum || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, errors.New("invalid canonical base64url")
	}
	return decoded, nil
}

func (api *API) decodeError(w http.ResponseWriter, err error) {
	if errors.Is(err, errBodyTooLarge) {
		api.writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
		return
	}
	api.writeError(w, http.StatusBadRequest, "invalid_request", "request body is invalid")
}

func (api *API) serviceError(w http.ResponseWriter, operation string, err error) {
	switch {
	case errors.Is(err, share.ErrNotFound):
		api.writeError(w, http.StatusNotFound, "not_found", "share not found")
	case errors.Is(err, share.ErrExpired):
		api.writeError(w, http.StatusGone, "expired", "share expired")
	case errors.Is(err, share.ErrUnauthorized):
		api.unauthorized(w)
	case errors.Is(err, share.ErrExpiration), errors.Is(err, share.ErrEmptyPatch):
		api.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, share.ErrInvalidText):
		api.writeError(w, http.StatusBadRequest, "invalid_request", "text content is invalid")
	case errors.Is(err, share.ErrInvalidEncryptedText):
		api.writeError(w, http.StatusBadRequest, "invalid_request", "encrypted text payload is invalid")
	default:
		api.log.Error("request failed", "operation", operation, "error", err)
		api.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (api *API) writeError(w http.ResponseWriter, status int, code, message string) {
	response := errorResponse{}
	response.Error.Code = code
	response.Error.Message = message
	api.writeJSON(w, status, response)
}

type textResponse struct {
	Format  domain.TextFormat `json:"format"`
	Content string            `json:"content"`
}

type encryptedTextResponse struct {
	Protocol   string `json:"protocol"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type responseShare struct {
	ID            string                 `json:"id"`
	PayloadKind   domain.PayloadKind     `json:"payload_kind"`
	PrivacyMode   domain.PrivacyMode     `json:"privacy_mode"`
	Text          *textResponse          `json:"text,omitempty"`
	EncryptedText *encryptedTextResponse `json:"encrypted_text,omitempty"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
	ExpiresAt     *string                `json:"expires_at"`
}

type shareResponse struct {
	Share responseShare `json:"share"`
}

type createResponse struct {
	Share      responseShare `json:"share"`
	OwnerToken string        `json:"owner_token"`
}

func responseFromShare(value share.Share) responseShare {
	response := responseShare{
		ID:          value.ID.String(),
		PayloadKind: value.PayloadKind,
		PrivacyMode: value.PrivacyMode,
		CreatedAt:   value.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if value.Text != nil {
		response.Text = &textResponse{Format: value.Text.Format, Content: value.Text.Content}
	}
	if value.EncryptedText != nil {
		response.EncryptedText = &encryptedTextResponse{
			Protocol:   value.EncryptedText.Protocol,
			Nonce:      base64.RawURLEncoding.EncodeToString(value.EncryptedText.Nonce),
			Ciphertext: base64.RawURLEncoding.EncodeToString(value.EncryptedText.Ciphertext),
		}
	}
	if value.ExpiresAt != nil {
		expiresAt := value.ExpiresAt.UTC().Format(time.RFC3339Nano)
		response.ExpiresAt = &expiresAt
	}
	return response
}

func (api *API) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := jsonv1.NewEncoder(w).Encode(value); err != nil {
		api.log.Error("write response failed", "error", err)
	}
}

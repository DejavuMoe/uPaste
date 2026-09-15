package maintenance

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/DejavuMoe/uPaste/internal/objectstore"
)

const (
	Interval  = 15 * time.Minute
	Grace     = 30 * time.Minute
	BatchSize = 256
	MaxPerRun = 2048
)

type Result struct {
	Purged, FileObjectsDeleted, FileCleanupFailures int
	Backlog                                         bool
	objectstore.ReconcileResult
}
type Service struct {
	db    *sql.DB
	store objectstore.Store
}

func New(db *sql.DB, store objectstore.Store) *Service { return &Service{db: db, store: store} }

// CleanupExpired reuses the established purge/reconciliation pass for the
// admin cleanup action and returns only the number of purged Shares.
func (service *Service) CleanupExpired(ctx context.Context, now time.Time) (int, error) {
	result, err := service.RunOnce(ctx, now)
	if err != nil {
		return 0, err
	}
	return result.Purged, nil
}

func (service *Service) RunOnce(ctx context.Context, now time.Time) (result Result, err error) {
	for range MaxPerRun / BatchSize {
		keys, count, batchErr := service.purgeBatch(ctx, now)
		if batchErr != nil {
			return result, batchErr
		}
		result.Purged += count
		for _, key := range keys {
			if deleteErr := service.store.Delete(key); deleteErr != nil {
				result.FileCleanupFailures++
			} else {
				result.FileObjectsDeleted++
			}
		}
		if count < BatchSize {
			break
		}
	}
	var remaining int
	if err := service.db.QueryRowContext(ctx, "SELECT count(*) FROM shares WHERE expires_at IS NOT NULL AND expires_at <= ?", now.UnixMilli()).Scan(&remaining); err != nil {
		return result, err
	}
	result.Backlog = remaining > 0
	references := make(map[string]struct{})
	rows, err := service.db.QueryContext(ctx, "SELECT storage_key FROM file_payloads")
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return result, err
		}
		references[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if reconciler, ok := service.store.(objectstore.Reconciler); ok {
		reconciled, reconcileErr := reconciler.Reconcile(ctx, references, now.Add(-Grace))
		result.ReconcileResult = reconciled
		if reconcileErr != nil {
			return result, reconcileErr
		}
	}
	return result, nil
}

func (service *Service) purgeBatch(ctx context.Context, now time.Time) ([]string, int, error) {
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT s.id, f.storage_key FROM shares s LEFT JOIN file_payloads f ON f.share_id = s.id WHERE s.expires_at IS NOT NULL AND s.expires_at <= ? ORDER BY s.id LIMIT ?`, now.UnixMilli(), BatchSize)
	if err != nil {
		return nil, 0, err
	}
	var items []struct{ id, key string }
	for rows.Next() {
		var item struct{ id, key string }
		var key sql.NullString
		if err := rows.Scan(&item.id, &key); err != nil {
			rows.Close()
			return nil, 0, err
		}
		item.key = key.String
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	keys := make([]string, 0)
	for _, item := range items {
		deleted, err := tx.ExecContext(ctx, "DELETE FROM shares WHERE id = ? AND expires_at IS NOT NULL AND expires_at <= ?", item.id, now.UnixMilli())
		if err != nil {
			return nil, 0, err
		}
		affected, err := deleted.RowsAffected()
		if err != nil {
			return nil, 0, err
		}
		if affected == 1 && item.key != "" {
			keys = append(keys, item.key)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	return keys, len(items), nil
}

func Start(ctx context.Context, service *Service, now func() time.Time, log *slog.Logger) {
	run := func() {
		started := time.Now()
		result, err := service.RunOnce(ctx, now())
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("maintenance failed", "error", err)
			return
		}
		if err == nil {
			log.Info("maintenance complete", "purged", result.Purged, "file_cleanup_failures", result.FileCleanupFailures, "orphans_deleted", result.OrphansDeleted, "stages_deleted", result.StagesDeleted, "missing", result.MissingReferenced, "anomalies", result.Anomalies, "backlog", result.Backlog, "duration", time.Since(started))
		}
	}
	run()
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

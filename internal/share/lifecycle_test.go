package share

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
)

// TestConcurrentLifecycleStress exercises create, read, owner update, delete,
// and independent creates together and then checks SQLite integrity.
func TestConcurrentLifecycleStress(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db, time.Now)
	ctx := context.Background()

	const count = 12
	values := make([]Share, 0, count)
	tokens := make([]capability.OwnerToken, 0, count)
	for range count {
		value, token, err := service.Create(ctx, CreateInput{Text: Text{Format: domain.TextPlain, Content: "lifecycle"}})
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
		tokens = append(tokens, token)
	}

	errs := make(chan error, count*4)
	var group sync.WaitGroup
	run := func(operation func() error) {
		group.Add(1)
		go func() {
			defer group.Done()
			errs <- operation()
		}()
	}

	for i := range count {
		value := values[i]
		run(func() error {
			_, err := service.Get(ctx, value.ID)
			return err
		})
	}
	for range count {
		run(func() error {
			_, _, err := service.Create(ctx, CreateInput{Text: Text{Format: domain.TextPlain, Content: "concurrent"}})
			return err
		})
	}
	for i := 0; i < 4; i++ {
		value, token := values[i], tokens[i]
		run(func() error {
			_, err := service.Update(ctx, value.ID, token.Reveal(), Patch{Text: &Text{Format: domain.TextSource, Content: "updated"}})
			return err
		})
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent lifecycle write: %v", err)
		}
	}

	// Delete the first four while reading the remaining eight.
	errs = make(chan error, count)
	for i := 0; i < 4; i++ {
		value, token := values[i], tokens[i]
		run(func() error {
			return service.Delete(ctx, value.ID, token.Reveal())
		})
	}
	for i := 4; i < count; i++ {
		value := values[i]
		run(func() error {
			_, err := service.Get(ctx, value.ID)
			return err
		})
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent lifecycle delete/read: %v", err)
		}
	}

	for i := 0; i < 4; i++ {
		if _, err := service.Get(ctx, values[i].ID); err == nil {
			t.Fatal("deleted Share remained readable")
		}
	}
	for i := 4; i < count; i++ {
		if _, err := service.Get(ctx, values[i].ID); err != nil {
			t.Fatalf("surviving Share became unreadable: %v", err)
		}
	}

	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q, error = %v", integrity, err)
	}
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign_key_check reported a violation")
	}
}

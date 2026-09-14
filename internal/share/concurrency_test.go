package share

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
)

func TestConcurrentOwnerUpdatesAndIndependentCreates(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db, time.Now)
	created, token, err := service.Create(context.Background(), CreateInput{Text: Text{Format: domain.TextPlain, Content: "initial"}})
	if err != nil {
		t.Fatal(err)
	}

	const workers = 4
	start := make(chan struct{})
	errs := make(chan error, workers*2)
	var group sync.WaitGroup
	for i := range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := service.Update(context.Background(), created.ID, token.Reveal(), Patch{
				Text: &Text{Format: domain.TextPlain, Content: string(rune('a' + i))},
			})
			errs <- err
		}()
	}
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, _, err := service.Create(context.Background(), CreateInput{Text: Text{Format: domain.TextPlain, Content: "new"}})
			errs <- err
		}()
	}
	close(start)
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent write: %v", err)
		}
	}
}

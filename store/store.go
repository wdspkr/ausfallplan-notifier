package store

import (
	"context"

	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
)

// Store persists and retrieves a Snapshot.
type Store interface {
	Load(ctx context.Context) (ausfallplan.Snapshot, error)
	Save(ctx context.Context, snap ausfallplan.Snapshot) error
}

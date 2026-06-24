package consent

import (
	"context"
	"errors"
)

// ErrNotFound is returned by a Repository when no consent matches the id.
var ErrNotFound = errors.New("consent not found")

// Repository is the persistence port for consents. Both the in-memory and the
// Postgres implementations satisfy it, and the service layer depends only on
// this interface — so the same business-logic tests run against either store.
type Repository interface {
	Create(ctx context.Context, c *Consent) error
	Get(ctx context.Context, id string) (*Consent, error)
	Update(ctx context.Context, c *Consent) error
	Delete(ctx context.Context, id string) error
}

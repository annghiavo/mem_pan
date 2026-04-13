package sqlc

// Store groups all generated query methods for higher layers.
type Store interface {
	Querier
}

var _ Store = (*Queries)(nil)

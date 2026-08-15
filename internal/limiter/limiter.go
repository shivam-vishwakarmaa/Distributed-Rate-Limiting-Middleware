package limiter

import "context"

type Result struct {
	Allowed   bool
	Remaining int
}

// CheckRequest encapsulates the parameters needed by various rate limiters
type CheckRequest struct {
	RouteID         string
	ClientID        string
	Limit           int
	WindowSeconds   int
	Capacity        int
	RefillPerSecond int
}

type Limiter interface {
	Allow(ctx context.Context, req CheckRequest) (*Result, error)
}

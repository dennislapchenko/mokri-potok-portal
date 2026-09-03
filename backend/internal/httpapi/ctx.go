package httpapi

import "context"

// nil2 is the background context for startup-time queries (bootstrap code).
func nil2() context.Context { return context.Background() }

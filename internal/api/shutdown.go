package api

import (
	"context"
	"net/http"
)

type shutdownKey struct{}

// ShutdownContext is an http.Server's BaseContext: long-lived streams (the events stream and
// Watch Together rooms) end when done closes. Close it as the server starts shutting down (for
// example from RegisterOnShutdown): http.Server.Shutdown waits for active connections, and an
// open browser tab would otherwise hold every restart for the whole shutdown timeout (and
// Docker kills a container ten seconds after asking it to stop). Ordinary requests are left to
// finish as before.
func ShutdownContext(done <-chan struct{}) context.Context {
	return context.WithValue(context.Background(), shutdownKey{}, done)
}

// shuttingDown is closed when the server serving r has begun shutting down; nil (never ready)
// when the server wasn't given a ShutdownContext.
func shuttingDown(r *http.Request) <-chan struct{} {
	done, _ := r.Context().Value(shutdownKey{}).(<-chan struct{})
	return done
}

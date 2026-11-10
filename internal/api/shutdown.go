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
	return ShutdownContextWithParent(context.Background(), done)
}

// shuttingDown is closed when the server serving r has begun shutting down; nil (never ready)
// when the server wasn't given a ShutdownContext.
func shuttingDown(r *http.Request) <-chan struct{} {
	done, _ := r.Context().Value(shutdownKey{}).(<-chan struct{})
	return done
}

// ShutdownContextWithParent cancels ordinary requests only on process shutdown.
// Listener replacement still ends streams through the separate done signal.
func ShutdownContextWithParent(parent context.Context, done <-chan struct{}) context.Context {
	return context.WithValue(parent, shutdownKey{}, done)
}

// TrackRequests keeps request admission and completion inside process shutdown.
func TrackRequests(next http.Handler, begin func() (func(), bool)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release, ok := begin()
		if !ok {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusServiceUnavailable, "server is stopping")
			return
		}
		defer release()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) backgroundContext() context.Context {
	if s.workContext != nil {
		return s.workContext
	}
	return context.Background()
}

// launchWork is invoked synchronously by a tracked request before it returns.
// The process joins requests before waiting for the asynchronous worker group.
func (s *Server) launchWork(run func(context.Context)) bool {
	root := s.backgroundContext()
	if root.Err() != nil {
		return false
	}
	if s.workers != nil {
		s.workers.Add(1)
	}
	go func() {
		if s.workers != nil {
			defer s.workers.Done()
		}
		run(root)
	}()
	return true
}

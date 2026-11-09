package settings

import "errors"

// ErrConcurrentChange means the validated snapshot is stale; the caller must rebuild its patch
// from current settings instead of overwriting another successful save or restored configuration.
var ErrConcurrentChange = errors.New("settings changed while being checked")

package metadata

import "errors"

// RequestError identifies a failure at the provider HTTP boundary, distinct from a store or
// application error later in matching. Cancellation/not-found identity remains available.
type RequestError struct {
	Provider string
	Err      error
}

func (e *RequestError) Error() string { return e.Err.Error() }
func (e *RequestError) Unwrap() error { return e.Err }
func requestError(provider string, err error) error {
	if err == nil || errors.Is(err, ErrNotFound) {
		return err
	}
	var existing *RequestError
	if errors.As(err, &existing) {
		return err
	}
	return &RequestError{Provider: provider, Err: err}
}

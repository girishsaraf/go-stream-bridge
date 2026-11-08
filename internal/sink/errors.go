package sink

import "errors"

// PermanentError marks a sink failure that will not succeed if retried, such as
// a schema violation, an authorization failure, or invalid input.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }

// Unwrap exposes the underlying error.
func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent wraps err as a permanent failure. Nil stays nil. Wrapping an
// already-permanent error is a no-op.
func Permanent(err error) error {
	if err == nil || IsPermanent(err) {
		return err
	}
	return &PermanentError{Err: err}
}

// IsPermanent reports whether err is or wraps a PermanentError.
func IsPermanent(err error) bool {
	if err == nil {
		return false
	}
	var target *PermanentError
	return errors.As(err, &target)
}

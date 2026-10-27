// Package failure adds safe operation labels without depending on transport or DB drivers.
package failure

type OperationError struct {
	Operation string
	Cause     error
}

func (e *OperationError) Error() string { return e.Operation + ": operation failed" }
func (e *OperationError) Unwrap() error { return e.Cause }

func Wrap(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &OperationError{Operation: operation, Cause: err}
}

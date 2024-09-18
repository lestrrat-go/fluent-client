package fluent

//nolint:errname
type errBufferFull struct{}
type errBufferFuller interface {
	BufferFull() bool
}
type causer interface {
	Cause() error
}

// Just need one instance
var errBufferFullInstance errBufferFull

// IsBufferFull returns true if the error is a BufferFull error
func IsBufferFull(e error) bool {
	for e != nil {
		if berr, ok := e.(errBufferFuller); ok {
			return berr.BufferFull()
		}

		if cerr, ok := e.(causer); ok {
			e = cerr.Cause()
		}
	}
	return false
}

func (e *errBufferFull) BufferFull() bool {
	return true
}

func (e *errBufferFull) Error() string {
	return `buffer full`
}

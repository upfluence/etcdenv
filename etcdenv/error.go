package etcdenv

const (
	ErrKeyNotFound    = 100
	ErrAlreadyStarted = iota
	ErrNotStarted
)

var (
	errorMap = map[int]string{
		ErrAlreadyStarted: "The process is already started",
		ErrNotStarted:     "The process is not started yet",
	}
)

type EtcdenvError struct {
	ErrorCode int
	Message   string
}

func (e EtcdenvError) Error() string {
	return e.Message
}

func newError(errorCode int) *EtcdenvError {
	return &EtcdenvError{ErrorCode: errorCode, Message: errorMap[errorCode]}
}

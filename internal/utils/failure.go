package utils

type ErrorType int

const (
	ErrInternal ErrorType = iota
	ErrBadInput
	ErrNotAllowed
	ErrNotFound
	ErrTooLarge
)

type Failure struct {
	Type    ErrorType
	Message string
	Data    map[string]any
}

func (f Failure) Error() string {
	return f.Message
}

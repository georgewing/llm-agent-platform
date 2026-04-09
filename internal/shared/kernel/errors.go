package kernel

import "fmt"

type DomainError struct {
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewNotFoundError(entity, id string) *DomainError {
	return &DomainError{Code: "NOT_FOUND", Message: fmt.Sprintf("%s not found: %s", entity, id)}
}

func NewValidationError(msg string) *DomainError {
	return &DomainError{Code: "VALIDATION_ERROR", Message: msg}
}

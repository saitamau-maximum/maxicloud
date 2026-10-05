package domain

import "errors"

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func IsValidationError(err error) bool {
	var ve ValidationError
	return errors.As(err, &ve)
}

type ForbiddenError struct {
	Message string
}

func (e ForbiddenError) Error() string {
	return e.Message
}

func IsForbiddenError(err error) bool {
	var fe ForbiddenError
	return errors.As(err, &fe)
}

type NotFoundError struct {
	Message string
}

func (e NotFoundError) Error() string {
	return e.Message
}

func IsNotFoundError(err error) bool {
	var ne NotFoundError
	return errors.As(err, &ne)
}

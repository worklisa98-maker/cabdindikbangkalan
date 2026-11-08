package services

type userFacingValidationError struct {
	message string
}

func (e *userFacingValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func newUserFacingValidationError(message string) error {
	return &userFacingValidationError{message: message}
}

func ValidationMessage(err error) (string, bool) {
	target, ok := err.(*userFacingValidationError)
	if !ok || target == nil {
		return "", false
	}
	return target.message, true
}

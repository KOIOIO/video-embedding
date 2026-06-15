package videoapp

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func InvalidArgumentError(message string) error {
	return ValidationError{Message: message}
}

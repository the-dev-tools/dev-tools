package customerror

type CustomError struct {
	userError     string
	internalError string
}

func New(userError, internalError string) CustomError {
	return CustomError{
		userError:     userError,
		internalError: internalError,
	}
}

func (c CustomError) Error() string {
	return c.internalError
}

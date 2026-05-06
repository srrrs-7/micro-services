package utilhttp

type ErrorType int

const (
	ErrNotFound ErrorType = iota
	ErrBadRequest
	ErrInternalServer
	ErrUnauthorized
	ErrForbidden
	ErrConflict
	ErrTooManyRequests
	ErrDatabase
)

func (e ErrorType) String() string {
	switch e {
	case ErrNotFound:
		return "not found"
	case ErrBadRequest:
		return "bad request"
	case ErrInternalServer:
		return "internal server error"
	case ErrUnauthorized:
		return "unauthorized"
	case ErrForbidden:
		return "forbidden"
	case ErrConflict:
		return "conflict"
	case ErrTooManyRequests:
		return "too many requests"
	case ErrDatabase:
		return "database error"
	default:
		return "unknown error"
	}
}

type AppError struct {
	Type    ErrorType
	Message string
}

// Error() を AppError に持たせる → 各型で実装不要
func (e AppError) Error() string {
	return e.Message
}

func newAppError(t ErrorType, err error) AppError {
	return AppError{Type: t, Message: err.Error()}
}

type NotFoundError struct{ AppError }

func NewNotFoundError(err error) NotFoundError {
	return NotFoundError{newAppError(ErrNotFound, err)}
}

type BadRequestError struct{ AppError }

func NewBadRequestError(err error) BadRequestError {
	return BadRequestError{newAppError(ErrBadRequest, err)}
}

type InternalServerError struct{ AppError }

func NewInternalServerError(err error) InternalServerError {
	return InternalServerError{newAppError(ErrInternalServer, err)}
}

type UnauthorizedError struct{ AppError }

func NewUnauthorizedError(err error) UnauthorizedError {
	return UnauthorizedError{newAppError(ErrUnauthorized, err)}
}

type ForbiddenError struct{ AppError }

func NewForbiddenError(err error) ForbiddenError {
	return ForbiddenError{newAppError(ErrForbidden, err)}
}

type ConflictError struct{ AppError }

func NewConflictError(err error) ConflictError {
	return ConflictError{newAppError(ErrConflict, err)}
}

type TooManyRequestsError struct{ AppError }

func NewTooManyRequestsError(err error) TooManyRequestsError {
	return TooManyRequestsError{newAppError(ErrTooManyRequests, err)}
}

type DBError struct{ AppError }

func NewDBError(err error) DBError {
	return DBError{newAppError(ErrDatabase, err)}
}

package utilhttp

import (
	"net/http"

	"github.com/goccy/go-json"
)

const (
	CONTENT_TYPE     = "Content-Type"
	APPLICATION_JSON = "application/json"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse[T any] struct {
	Data T `json:"data"`
}

type Response interface {
	isResponse()
}

func (e ErrorResponse) isResponse()      {}
func (s SuccessResponse[T]) isResponse() {}

func writeJSON(w http.ResponseWriter, statusCode int, msg Response) {
	w.Header().Set(CONTENT_TYPE, APPLICATION_JSON)
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(msg); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to encode response"}`))
	}
}

func ResponseOk[T any](w http.ResponseWriter, msg T) {
	writeJSON(w, http.StatusOK, SuccessResponse[T]{Data: msg})
}

func ResponseAccepted[T any](w http.ResponseWriter, msg T) {
	writeJSON(w, http.StatusAccepted, SuccessResponse[T]{Data: msg})
}

func ResponseError(w http.ResponseWriter, err error) {
	// 各 typed wrapper は AppError を値埋め込みで保持しているだけで Unwrap を実装しないため、
	// errors.As では取り出せない。明示的な type switch で AppError を取り出す。
	var appErr AppError
	switch e := err.(type) {
	case AppError:
		appErr = e
	case NotFoundError:
		appErr = e.AppError
	case BadRequestError:
		appErr = e.AppError
	case InternalServerError:
		appErr = e.AppError
	case UnauthorizedError:
		appErr = e.AppError
	case ForbiddenError:
		appErr = e.AppError
	case ConflictError:
		appErr = e.AppError
	case TooManyRequestsError:
		appErr = e.AppError
	case DBError:
		appErr = e.AppError
	default:
		responseInternalServerError(w, err)
		return
	}

	switch appErr.Type {
	case ErrNotFound:
		responseNotFound(w, appErr)
	case ErrBadRequest:
		responseBadRequest(w, appErr)
	case ErrUnauthorized:
		responseUnauthorized(w, appErr)
	case ErrForbidden:
		responseForbidden(w, appErr)
	case ErrConflict:
		responseConflict(w, appErr)
	case ErrTooManyRequests:
		responseTooManyRequests(w, appErr)
	case ErrInternalServer, ErrDatabase:
		responseInternalServerError(w, appErr)
	default:
		responseInternalServerError(w, appErr)
	}
}

func responseInternalServerError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
}

func responseBadRequest(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
}

func responseNotFound(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
}

func responseUnauthorized(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
}

func responseForbidden(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusForbidden, ErrorResponse{Error: err.Error()})
}

func responseConflict(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error()})
}

func responseTooManyRequests(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: err.Error()})
}

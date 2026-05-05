package utilhttp

import (
	"errors"
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
	var (
		notFoundErr       NotFoundError
		badRequestErr     BadRequestError
		internalServerErr InternalServerError
		unauthorizedErr   UnauthorizedError
		forbiddenErr      ForbiddenError
		conflictErr       ConflictError
		tooManyReqErr     TooManyRequestsError
	)

	switch {
	case errors.As(err, &notFoundErr):
		responseNotFound(w, notFoundErr)
	case errors.As(err, &badRequestErr):
		responseBadRequest(w, badRequestErr)
	case errors.As(err, &internalServerErr):
		responseInternalServerError(w, internalServerErr)
	case errors.As(err, &unauthorizedErr):
		responseUnauthorized(w, unauthorizedErr)
	case errors.As(err, &forbiddenErr):
		responseForbidden(w, forbiddenErr)
	case errors.As(err, &conflictErr):
		responseConflict(w, conflictErr)
	case errors.As(err, &tooManyReqErr):
		responseTooManyRequests(w, tooManyReqErr)
	default:
		responseInternalServerError(w, err)
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

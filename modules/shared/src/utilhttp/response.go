package utilhttp

import (
	"net/http"

	"github.com/goccy/go-json"
)

const (
	CONTENT_TYPE     = "Content-Type"
	APPLICATION_JSON = "application/json"
)

func Json[T any](v T) ([]byte, error) {
	j, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return j, nil
}

func ResponseOk(w http.ResponseWriter, msg []byte) {
	w.Header().Set(CONTENT_TYPE, APPLICATION_JSON)
	w.WriteHeader(http.StatusOK)
	w.Write(msg)
}

func ResponseInternalServerError(w http.ResponseWriter, msg []byte) {
	w.Header().Set(CONTENT_TYPE, APPLICATION_JSON)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(msg)
}

func ResponseBadRequest(w http.ResponseWriter, msg []byte) {
	w.Header().Set(CONTENT_TYPE, APPLICATION_JSON)
	w.WriteHeader(http.StatusBadRequest)
	w.Write(msg)
}

func ResponseNotFound(w http.ResponseWriter, msg []byte) {
	w.Header().Set(CONTENT_TYPE, APPLICATION_JSON)
	w.WriteHeader(http.StatusBadRequest)
	w.Write(msg)
}

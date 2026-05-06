// request.go
package utilhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type contextKey string

func SetContextValue(ctx context.Context, key contextKey, val any) context.Context {
	return context.WithValue(ctx, key, val)
}

type Validator interface {
	Validate() error
}

func RequestUrlParam[T Validator](req *http.Request, key contextKey) (T, error) {
	var zero T

	t, ok := req.Context().Value(key).(T)
	if !ok {
		return zero, BadRequestError{Message: fmt.Sprintf("missing url param %q", key)}
	}
	if err := t.Validate(); err != nil {
		return zero, BadRequestError{Message: fmt.Sprintf("invalid url param %q: %v", key, err)}
	}
	return t, nil
}

func RequestBody[T Validator](req *http.Request) (T, error) {
	var zero T

	b, err := io.ReadAll(req.Body)
	if err != nil {
		return zero, BadRequestError{Message: fmt.Sprintf("read request body: %v", err)}
	}
	defer req.Body.Close()

	var body T
	if err := json.Unmarshal(b, &body); err != nil {
		return zero, BadRequestError{Message: fmt.Sprintf("unmarshal request body: %v", err)}
	}

	if err := body.Validate(); err != nil {
		return zero, BadRequestError{Message: fmt.Sprintf("invalid request body: %v", err)}
	}

	return body, nil
}

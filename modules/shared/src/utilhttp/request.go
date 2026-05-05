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

func RequestUrlParam[T comparable](req *http.Request, key contextKey) (T, error) {
	t, ok := req.Context().Value(key).(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("url param %q not found or type mismatch", key)
	}
	return t, nil
}

func RequestBody[T any](req *http.Request) (T, error) {
	var zero T

	b, err := io.ReadAll(req.Body)
	if err != nil {
		return zero, fmt.Errorf("read request body: %w", err)
	}
	defer req.Body.Close()

	var body T
	if err := json.Unmarshal(b, &body); err != nil {
		return zero, fmt.Errorf("unmarshal request body: %w", err)
	}

	return body, nil
}

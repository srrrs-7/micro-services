package response

import (
	"net/http"
	"shared/utilhttp"
)

func ResponseOK[T any](w http.ResponseWriter, msg T) error {
	res, err := utilhttp.Json(msg)
	if err != nil {
		return err
	}
	utilhttp.ResponseOk(w, res)
	return nil
}

func ResponseAccepted[T any](w http.ResponseWriter, msg T) error {
	res, err := utilhttp.Json(msg)
	if err != nil {
		return err
	}
	utilhttp.ResponseAccepted(w, res)
	return nil
}

func ResponseBadRequest[T any](w http.ResponseWriter, msg T) error {
	res, err := utilhttp.Json(msg)
	if err != nil {
		return err
	}
	utilhttp.ResponseBadRequest(w, res)
	return nil
}

func ResponseNotFound[T any](w http.ResponseWriter, msg T) error {
	res, err := utilhttp.Json(msg)
	if err != nil {
		return err
	}
	utilhttp.ResponseNotFound(w, res)
	return nil
}

func ResponseInternalServerError[T any](w http.ResponseWriter, msg T) error {
	res, err := utilhttp.Json(msg)
	if err != nil {
		return err
	}
	utilhttp.ResponseInternalServerError(w, res)
	return nil
}

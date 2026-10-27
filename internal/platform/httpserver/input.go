package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
)

var ErrInvalidRequest = errors.New("invalid HTTP request")

// DecodeJSON enforces one strict JSON value. Each feature retains its existing
// body limit and translates this transport error to its own domain sentinel.
func DecodeJSON(w http.ResponseWriter, r *http.Request, value any, maximumBytes int64) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || maximumBytes <= 0 {
		return ErrInvalidRequest
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximumBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(value); err != nil {
		return ErrInvalidRequest
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ErrInvalidRequest
	}
	return nil
}

func Pagination(r *http.Request) (int, int, error) {
	limit, offset := 50, 0
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, ErrInvalidRequest
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, ErrInvalidRequest
		}
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return 0, 0, ErrInvalidRequest
	}
	return limit, offset, nil
}

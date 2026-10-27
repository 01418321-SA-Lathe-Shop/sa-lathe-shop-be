package httpserver

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStrictJSONAndFeatureBodyLimits(t *testing.T) {
	for _, test := range []struct {
		name, body, media string
		maximum           int64
		valid             bool
	}{
		{"one value", `{"name":"ช่าง"}`, "application/json; charset=utf-8", 8192, true},
		{"trailing whitespace", `{"name":"x"}  `, "application/json", 8192, true},
		{"second value", `{"name":"x"} {}`, "application/json", 8192, false},
		{"unknown field", `{"role":"admin"}`, "application/json", 8192, false},
		{"wrong media", `{}`, "text/plain", 8192, false},
		{"missing media", `{}`, "", 8192, false},
		{"auth body limit", `{"name":"` + strings.Repeat("x", 9000) + `"}`, "application/json", 8192, false},
		{"feature body limit", `{"name":"` + strings.Repeat("x", 9000) + `"}`, "application/json", 65536, true},
		{"empty", "", "application/json", 8192, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.media)
			var value struct {
				Name string `json:"name"`
			}
			err := DecodeJSON(httptest.NewRecorder(), request, &value, test.maximum)
			if (err == nil) != test.valid || (err != nil && !errors.Is(err, ErrInvalidRequest)) {
				t.Fatal("strict decoding contract changed", err)
			}
		})
	}
}

func TestSharedPagination(t *testing.T) {
	for _, test := range []struct {
		query         string
		limit, offset int
		valid         bool
	}{
		{"", 50, 0, true}, {"?limit=1&offset=2", 1, 2, true},
		{"?limit=100", 100, 0, true}, {"?limit=101", 0, 0, false},
		{"?limit=0", 0, 0, false}, {"?offset=-1", 0, 0, false},
		{"?limit=abc", 0, 0, false}, {"?offset=99999999999999999999999", 0, 0, false},
	} {
		limit, offset, err := Pagination(httptest.NewRequest("GET", "/"+test.query, nil))
		if (err == nil) != test.valid || (err == nil && (limit != test.limit || offset != test.offset)) {
			t.Fatal("pagination contract changed", test.query, limit, offset, err)
		}
	}
}

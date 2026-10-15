package quotation

import (
	"context"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerRejectsInvalidAndSpoofedJSON(t *testing.T) {
	// Authenticate middleware deliberately leaves an anonymous context. Valid
	// input must then fail at the service permission boundary, not reach SQL.
	f := &fakeRepo{}
	router := chi.NewRouter()
	NewHandler(NewService(f)).Routes(router, func(next http.Handler) http.Handler { return next })
	for _, test := range []struct {
		body   string
		status int
	}{
		{`{"material_cost":100,"labor_cost":"20","final_price":"200"}`, 400},
		{`{"material_cost":"100.001","labor_cost":"20","final_price":"200"}`, 400},
		{`{"material_cost":"100","labor_cost":"20","final_price":"200","role":"worker"}`, 400},
		{`{"material_cost":"100","labor_cost":"20","final_price":"200"} {}`, 400},
		{`{"material_cost":"100","labor_cost":"20","final_price":"200"}`, 403},
	} {
		r := httptest.NewRequest("POST", "/requests/"+testID+"/quotations", strings.NewReader(test.body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("expected %d got %d", test.status, w.Code)
		}
	}
	if f.calls != 0 {
		t.Fatal("invalid or anonymous request touched repository")
	}
}
func TestEstimateNoDatabaseCall(t *testing.T) {
	f := &fakeRepo{}
	in := input(t, "1600")
	in.FinalPrice = nil
	result, err := NewService(f).Estimate(context.Background(), worker(), in)
	if err != nil || result.MinimumPrice.String() != "1600.00" || f.calls != 0 {
		t.Fatal(err)
	}
}

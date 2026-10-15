package quotation

import (
	"encoding/json"
	"errors"
	"testing"
)

func amount(t *testing.T, s string) Amount {
	t.Helper()
	a, err := ParseAmount(s)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestAmountParsing(t *testing.T) {
	for _, text := range []string{"", "-1", "+1", " 1", "1 ", "1e3", "NaN", ".01", "1.", "1.001", "10000000000000000", "9999999999999999.999", "1/2", "๑"} {
		if _, err := ParseAmount(text); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("accepted %q", text)
		}
	}
	for input, wanted := range map[string]string{"0": "0.00", "1": "1.00", "0.1": "0.10", "0001.02": "1.02", "9999999999999999.99": "9999999999999999.99"} {
		a, err := ParseAmount(input)
		if err != nil || a.String() != wanted {
			t.Fatalf("%s: %v", input, err)
		}
	}
}
func TestMinimumPrice(t *testing.T) {
	for _, test := range []struct{ material, labor, minimum string }{{"800", "200", "1600.00"}, {"0.01", "0.01", "0.04"}, {"0.01", "0", "0.02"}, {"0", "0", "0.00"}, {"1.23", "4.56", "9.27"}} {
		estimate, err := EstimatePrice(amount(t, test.material), amount(t, test.labor))
		if err != nil || estimate.MinimumPrice.String() != test.minimum {
			t.Fatalf("minimum %s: %v", test.minimum, err)
		}
	}
	if _, err := EstimatePrice(amount(t, "9999999999999999.99"), amount(t, "1")); err == nil {
		t.Fatal("overflowing sum accepted")
	}
	if _, err := EstimatePrice(amount(t, "9999999999999999.99"), Amount{}); err == nil {
		t.Fatal("unrepresentable minimum accepted")
	}
}
func TestAmountJSON(t *testing.T) {
	for _, data := range []string{`1.23`, `null`, `true`, `"1e2"`, `"1.234"`} {
		var a Amount
		if err := json.Unmarshal([]byte(data), &a); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	var a Amount
	if err := json.Unmarshal([]byte(`"0.01"`), &a); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(a)
	if string(encoded) != `"0.01"` {
		t.Fatal(string(encoded))
	}
}

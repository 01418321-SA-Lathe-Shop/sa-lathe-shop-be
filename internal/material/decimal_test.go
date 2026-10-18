package material

import (
	"errors"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"testing"
)

func decimal(t *testing.T, s string) Decimal {
	t.Helper()
	v, err := ParseDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestExactDecimalAndAverage(t *testing.T) {
	for _, s := range []string{"", "-1", "1e2", "1.0000001", "1000000000000", "NaN", "1.", ".2"} {
		if _, err := ParseDecimal(s); !errors.Is(err, req.ErrInvalidInput) {
			t.Fatal("invalid value", s)
		}
	}
	if decimal(t, "0001.2").String() != "1.200000" {
		t.Fatal("normalization")
	}
	avg, err := WeightedAverage(decimal(t, "2.5"), decimal(t, "10"), decimal(t, "2.5"), decimal(t, "20"))
	if err != nil || avg.String() != "15.000000" {
		t.Fatal(avg, err)
	}
	avg, err = WeightedAverage(decimal(t, "1"), decimal(t, "0"), decimal(t, "1"), decimal(t, "0.000001"))
	if err != nil || avg.String() != "0.000001" {
		t.Fatal("half-up rounding", avg, err)
	}
	max := decimal(t, "999999999999.999999")
	avg, err = WeightedAverage(Decimal{}, max, max, max)
	if err != nil || avg != max {
		t.Fatal("big integer overflow", avg, err)
	}
	reserved, shortage, err := Allocate(decimal(t, "10"), decimal(t, "3"), decimal(t, "2"))
	if err != nil || reserved.String() != "5.000000" || shortage.String() != "5.000000" {
		t.Fatal("partial allocation")
	}
	m := Material{OnHand: decimal(t, "10"), Reserved: decimal(t, "7"), AverageCost: decimal(t, "12")}
	target := decimal(t, "6")
	in := StockInput{IdempotencyKey: "00000000-0000-0000-0000-000000000001", Mode: "correction", Quantity: &target, Reason: "Count correction", ExpectedVersion: 1}
	if _, _, err = StockEffect(m, in); !errors.Is(err, ErrConflict) {
		t.Fatal("reserved stock correction allowed")
	}
	target = decimal(t, "8")
	_, cost, err := StockEffect(m, in)
	if err != nil || cost != m.AverageCost {
		t.Fatal("correction changed cost")
	}
}

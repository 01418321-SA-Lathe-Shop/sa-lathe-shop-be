package material

import (
	"encoding/json"
	"fmt"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"math/big"
	"strconv"
	"strings"
)

// Decimal matches NUMERIC(18,6): exact quantities and per-base-unit costs.
// Inventory values are not customer invoice totals. Never use float arithmetic.
type Decimal struct{ units int64 }

const MaxUnits int64 = 999999999999999999

func ParseDecimal(s string) (Decimal, error) {
	parts := strings.Split(s, ".")
	if len(parts) > 2 || len(parts[0]) < 1 || len(parts[0]) > 12 {
		return Decimal{}, req.ErrInvalidInput
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
		if len(frac) < 1 || len(frac) > 6 {
			return Decimal{}, req.ErrInvalidInput
		}
	}
	for _, r := range parts[0] + frac {
		if r < '0' || r > '9' {
			return Decimal{}, req.ErrInvalidInput
		}
	}
	raw := parts[0] + frac + strings.Repeat("0", 6-len(frac))
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v > MaxUnits {
		return Decimal{}, req.ErrInvalidInput
	}
	return Decimal{v}, nil
}
func (d Decimal) String() string               { return fmt.Sprintf("%d.%06d", d.units/1000000, d.units%1000000) }
func (d Decimal) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }
func (d *Decimal) UnmarshalJSON(raw []byte) error {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return req.ErrInvalidInput
	}
	v, err := ParseDecimal(s)
	if err == nil {
		*d = v
	}
	return err
}
func (d Decimal) IsZero() bool { return d.units == 0 }
func (d Decimal) Add(v Decimal) (Decimal, error) {
	if v.units < 0 || d.units > MaxUnits-v.units {
		return Decimal{}, req.ErrInvalidInput
	}
	return Decimal{d.units + v.units}, nil
}
func (d Decimal) Subtract(v Decimal) (Decimal, error) {
	if v.units < 0 || v.units > d.units {
		return Decimal{}, req.ErrInvalidInput
	}
	return Decimal{d.units - v.units}, nil
}

// Convert is exact: a result that does not fit NUMERIC(18,6) is rejected, never rounded.
func (d Decimal) Convert(factor Decimal) (Decimal, error) {
	scale := big.NewInt(1000000)
	product := new(big.Int).Mul(big.NewInt(d.units), big.NewInt(factor.units))
	quotient, remainder := new(big.Int).QuoRem(product, scale, new(big.Int))
	if remainder.Sign() != 0 || !quotient.IsInt64() || quotient.Int64() > MaxUnits {
		return Decimal{}, req.ErrInvalidInput
	}
	return Decimal{quotient.Int64()}, nil
}
func Allocate(required, previous, available Decimal) (Decimal, Decimal, error) {
	capacity, err := previous.Add(available)
	if err != nil {
		return Decimal{}, Decimal{}, err
	}
	reserved := required
	if capacity.units < required.units {
		reserved = capacity
	}
	return reserved, Decimal{required.units - reserved.units}, nil
}
func WeightedAverage(oldQty, oldCost, addedQty, unitCost Decimal) (Decimal, error) {
	total, err := oldQty.Add(addedQty)
	if err != nil || addedQty.IsZero() {
		return Decimal{}, req.ErrInvalidInput
	}
	a := new(big.Int).Mul(big.NewInt(oldQty.units), big.NewInt(oldCost.units))
	b := new(big.Int).Mul(big.NewInt(addedQty.units), big.NewInt(unitCost.units))
	a.Add(a, b)
	divisor := big.NewInt(total.units)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(a, divisor, remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(divisor) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() || quotient.Int64() > MaxUnits {
		return Decimal{}, req.ErrInvalidInput
	}
	return Decimal{quotient.Int64()}, nil
}

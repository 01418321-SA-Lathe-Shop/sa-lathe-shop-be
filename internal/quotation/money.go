// Package quotation implements UC2 estimation and quotation rules.
package quotation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidInput = errors.New("invalid quotation input")

// NUMERIC(18,2): 16 whole digits plus two decimals. No floating point is used.
const MaxSatang int64 = 999999999999999999

type Amount struct{ satang int64 }

func ParseAmount(text string) (Amount, error) {
	if text == "" || len(text) > 19 {
		return Amount{}, ErrInvalidInput
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 || len(parts[0]) < 1 || len(parts[0]) > 16 {
		return Amount{}, ErrInvalidInput
	}
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return Amount{}, ErrInvalidInput
		}
	}
	fraction := "00"
	if len(parts) == 2 {
		if len(parts[1]) < 1 || len(parts[1]) > 2 {
			return Amount{}, ErrInvalidInput
		}
		fraction = parts[1]
		for _, c := range fraction {
			if c < '0' || c > '9' {
				return Amount{}, ErrInvalidInput
			}
		}
		if len(fraction) == 1 {
			fraction += "0"
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Amount{}, ErrInvalidInput
	}
	cents, _ := strconv.ParseInt(fraction, 10, 64)
	value := whole*100 + cents
	if value > MaxSatang {
		return Amount{}, ErrInvalidInput
	}
	return Amount{satang: value}, nil
}
func (a Amount) String() string               { return fmt.Sprintf("%d.%02d", a.satang/100, a.satang%100) }
func (a Amount) MarshalJSON() ([]byte, error) { return json.Marshal(a.String()) }
func (a *Amount) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return ErrInvalidInput
	}
	parsed, err := ParseAmount(value)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

type Estimate struct {
	MaterialCost Amount `json:"material_cost"`
	LaborCost    Amount `json:"labor_cost"`
	TotalCost    Amount `json:"total_cost"`
	MinimumPrice Amount `json:"minimum_price"`
}

func EstimatePrice(material, labor Amount) (Estimate, error) {
	total := material.satang + labor.satang
	if total < 0 || total > MaxSatang {
		return Estimate{}, ErrInvalidInput
	}
	// ceil(total * 8/5), avoiding a potentially overflowing multiplication.
	minimum := (total/5)*8 + (total%5*8+4)/5
	if minimum > MaxSatang {
		return Estimate{}, ErrInvalidInput
	}
	return Estimate{MaterialCost: material, LaborCost: labor, TotalCost: Amount{total}, MinimumPrice: Amount{minimum}}, nil
}

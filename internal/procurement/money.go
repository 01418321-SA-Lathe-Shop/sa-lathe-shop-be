package procurement

import (
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"math/big"
	"strings"
)

// PriceLine rounds the exact six-place quantity * cost half-up to one satang.
func PriceLine(quantity, cost m.Decimal) (q.Amount, error) {
	a, ok := new(big.Int).SetString(strings.ReplaceAll(quantity.String(), ".", ""), 10)
	if !ok {
		return q.Amount{}, req.ErrInvalidInput
	}
	b, ok := new(big.Int).SetString(strings.ReplaceAll(cost.String(), ".", ""), 10)
	if !ok {
		return q.Amount{}, req.ErrInvalidInput
	}
	product := new(big.Int).Mul(a, b)
	product.Add(product, big.NewInt(5000000000))
	product.Quo(product, big.NewInt(10000000000))
	return amountFromSatang(product)
}
func amountFromSatang(n *big.Int) (q.Amount, error) {
	text := n.String()
	if len(text) < 3 {
		text = strings.Repeat("0", 3-len(text)) + text
	}
	v, err := q.ParseAmount(text[:len(text)-2] + "." + text[len(text)-2:])
	if err != nil {
		return q.Amount{}, req.ErrInvalidInput
	}
	return v, nil
}
func Totals(lines []LineInput) ([]q.Amount, q.Amount, error) {
	values := make([]q.Amount, 0, len(lines))
	sum := new(big.Int)
	for _, line := range lines {
		if line.Quantity == nil || line.UnitCost == nil || line.Quantity.IsZero() {
			return nil, q.Amount{}, req.ErrInvalidInput
		}
		v, err := PriceLine(*line.Quantity, *line.UnitCost)
		if err != nil {
			return nil, q.Amount{}, err
		}
		n, _ := new(big.Int).SetString(strings.ReplaceAll(v.String(), ".", ""), 10)
		sum.Add(sum, n)
		values = append(values, v)
	}
	total, err := amountFromSatang(sum)
	return values, total, err
}

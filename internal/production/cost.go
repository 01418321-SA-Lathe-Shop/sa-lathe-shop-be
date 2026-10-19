package production

import (
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"math/big"
	"strings"
)

// IssueEffect consumes this job's reservation first, never another job's stock.
func IssueEffect(stock m.Material, own, quantity m.Decimal) (m.Decimal, m.Decimal, error) {
	if quantity.IsZero() {
		return m.Decimal{}, m.Decimal{}, req.ErrInvalidInput
	}
	used := quantity
	if _, e := quantity.Subtract(own); e == nil {
		used = own
	}
	extra, e := quantity.Subtract(used)
	if e != nil {
		return m.Decimal{}, m.Decimal{}, e
	}
	free, e := stock.OnHand.Subtract(stock.Reserved)
	if e != nil {
		return m.Decimal{}, m.Decimal{}, ErrConflict
	}
	if _, e = free.Subtract(extra); e != nil || (!stock.Active && !extra.IsZero()) {
		return m.Decimal{}, m.Decimal{}, ErrConflict
	}
	next, e := stock.OnHand.Subtract(quantity)
	if e != nil {
		return m.Decimal{}, m.Decimal{}, ErrConflict
	}
	return used, next, nil
}
func SumAmounts(values ...q.Amount) (q.Amount, error) {
	sum := new(big.Int)
	for _, v := range values {
		n, ok := new(big.Int).SetString(strings.ReplaceAll(v.String(), ".", ""), 10)
		if !ok {
			return q.Amount{}, req.ErrInvalidInput
		}
		sum.Add(sum, n)
	}
	text := sum.String()
	if len(text) < 3 {
		text = strings.Repeat("0", 3-len(text)) + text
	}
	v, e := q.ParseAmount(text[:len(text)-2] + "." + text[len(text)-2:])
	if e != nil {
		return q.Amount{}, req.ErrInvalidInput
	}
	return v, nil
}

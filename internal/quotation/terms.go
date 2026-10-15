package quotation

import "time"

// Terms is an acceptance snapshot, not a payment receipt or an open job.
type Terms struct {
	CustomerType        string    `json:"customer_type"`
	FinalPrice          Amount    `json:"final_price"`
	DepositRequired     bool      `json:"deposit_required"`
	PORequired          bool      `json:"po_required"`
	DepositAmount       Amount    `json:"deposit_amount"`
	BalanceAfterDeposit Amount    `json:"balance_after_deposit"`
	AcceptedAt          time.Time `json:"accepted_at"`
	DocumentsDueAt      time.Time `json:"documents_due_at"`
}

func AcceptanceTerms(customerType string, price Amount, acceptedAt time.Time) (Terms, error) {
	if (customerType != "individual" && customerType != "company") || acceptedAt.IsZero() {
		return Terms{}, ErrInvalidInput
	}
	acceptedAt = acceptedAt.UTC()
	due := acceptedAt.Add(336 * time.Hour)
	if acceptedAt.Year() < 1 || due.Year() > 9999 {
		return Terms{}, ErrInvalidInput
	}
	depositRequired := customerType == "individual" || price.satang > 1000000
	var deposit Amount
	if depositRequired {
		deposit = Amount{satang: (price.satang + 1) / 2}
	}
	return Terms{CustomerType: customerType, FinalPrice: price, DepositRequired: depositRequired, PORequired: customerType == "company", DepositAmount: deposit, BalanceAfterDeposit: Amount{satang: price.satang - deposit.satang}, AcceptedAt: acceptedAt, DocumentsDueAt: due}, nil
}

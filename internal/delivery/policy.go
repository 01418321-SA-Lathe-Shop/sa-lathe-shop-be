package delivery

import q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"

func CanApply(v State, in Action) error {
	if (v.RequestStatus != "ongoing" && !(v.RequestStatus == "completed" && in.Kind == "review" && in.Outcome == "rejected")) || v.Version != in.ExpectedVersion {
		return ErrConflict
	}
	switch in.Kind {
	case "terms":
		if v.JobStatus != "completed" || v.Invoice != nil || v.CreditDays != nil {
			return ErrConflict
		}
	case "invoice":
		if v.JobStatus != "completed" || v.Invoice != nil {
			return ErrConflict
		}
	case "ship":
		if v.JobStatus != "completed" || v.Invoice == nil {
			return ErrConflict
		}
		if len(v.Deliveries) > 0 && v.Deliveries[len(v.Deliveries)-1].Status != "returned" {
			return ErrConflict
		}
	case "accept", "return":
		if v.JobStatus != "completed" || len(v.Deliveries) == 0 || v.Deliveries[len(v.Deliveries)-1].Status != "shipping" {
			return ErrConflict
		}
	case "bank", "cash":
		if v.Invoice == nil || in.Amount == nil {
			return ErrConflict
		}
		if _, e := v.Invoice.Balance.Subtract(*in.Amount); e != nil {
			return ErrConflict
		}
	case "review":
		if v.Invoice == nil {
			return ErrConflict
		}
	default:
		return ErrConflict
	}
	return nil
}
func Remaining(total, paid q.Amount) (q.Amount, error) { return total.Subtract(paid) }
func ShouldClose(v State) bool {
	return v.Invoice != nil && v.Invoice.Balance.String() == "0.00" && len(v.Deliveries) > 0 && v.Deliveries[len(v.Deliveries)-1].Status == "delivered" && v.JobStatus == "completed"
}

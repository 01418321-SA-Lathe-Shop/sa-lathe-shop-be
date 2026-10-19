package production

import req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"

// NextStatus is pure business policy, evaluated inside repository row locks.
func NextStatus(state State, in Action) (string, error) {
	switch in.Kind {
	case "start":
		if state.Status != "queued" || state.PlanVersion != in.PlanVersion {
			return "", ErrConflict
		}
		nonempty := false
		for _, line := range state.Materials {
			if line.Required.IsZero() {
				continue
			}
			nonempty = true
			short, e := line.Required.Subtract(line.Reserved)
			if e != nil || !short.IsZero() {
				return "", ErrConflict
			}
		}
		if !nonempty {
			return "", ErrConflict
		}
		return "in_progress", nil
	case "use", "time":
		if state.Status != "in_progress" {
			return "", ErrConflict
		}
		return state.Status, nil
	case "complete":
		if state.Status != "in_progress" {
			return "", ErrConflict
		}
		return "completed", nil
	case "labor":
		if state.Status != "in_progress" && state.Status != "completed" {
			return "", ErrConflict
		}
		return state.Status, nil
	default:
		return "", req.ErrInvalidInput
	}
}

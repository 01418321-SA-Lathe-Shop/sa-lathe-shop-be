package auth

import (
	"bytes"
	"encoding/json"
)

// Explicit null is different from an omitted contact field: contacts cannot be
// cleared. Keep strict unknown-field decoding when using this custom decoder.
func (u *WorkerUpdate) UnmarshalJSON(data []byte) error {
	type plain WorkerUpdate
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return ErrInvalidInput
	}
	for _, key := range []string{"email", "phone"} {
		if raw, ok := fields[key]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return ErrInvalidInput
		}
	}
	var value plain
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return ErrInvalidInput
	}
	*u = WorkerUpdate(value)
	return nil
}

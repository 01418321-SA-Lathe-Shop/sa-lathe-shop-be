package production

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
)

// A response-shape characterization, not a substitute for real repository replay tests.
type contractRepository struct {
	Repository
	state State
}

func (r *contractRepository) ProductionState(context.Context, string, string) (State, error) {
	return r.state, nil
}
func (r *contractRepository) ApplyProduction(context.Context, string, string, Action) (State, error) {
	return r.state, nil
}

func TestLegacyProductionResponseContract(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000001"
	instant := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	amount, err := q.ParseAmount("12.34")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{auth.RoleAdmin, auth.RoleWorker} {
		t.Run(role, func(t *testing.T) {
			cost := qty("1.234567")
			repository := &contractRepository{state: State{
				Summary:     Summary{JobID: id, RequestID: id, Status: "completed", Version: 4, PlanVersion: 2, Cycle: 1, StartedAt: &instant, CompletedAt: &instant},
				Materials:   []MaterialState{},
				Usage:       []Usage{{ID: id, MaterialID: id, Quantity: qty("10"), UnitCost: &cost, Total: &amount, CreatedAt: instant}},
				TimeEntries: []TimeEntry{}, Events: []Event{}, Photos: []Photo{},
				Finance: &Finance{MaterialCost: amount, LaborCost: &amount, TotalCost: &amount},
			}}
			service := NewService(repository)
			actor := auth.Account{ID: id, Active: true, Role: role}
			value, err := service.Get(context.Background(), actor, id)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]json.RawMessage
			if err = json.Unmarshal(data, &body); err != nil {
				t.Fatal(err)
			}
			want := []string{"job_id", "request_id", "status", "version", "cycle", "material_plan_version", "started_at", "completed_at", "materials", "usage", "time_entries", "labor_minutes", "events", "photos"}
			if role == auth.RoleAdmin {
				want = append(want, "finance")
			}
			keys := make([]string, 0, len(body))
			for key := range body {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			sort.Strings(want)
			if !reflect.DeepEqual(keys, want) {
				t.Fatalf("legacy keys changed: got %v want %v", keys, want)
			}
			for _, field := range []string{"materials", "time_entries", "events", "photos"} {
				if string(body[field]) != "[]" {
					t.Fatalf("%s must remain an empty array, not omitted/null", field)
				}
			}
			var usage []map[string]json.RawMessage
			if err = json.Unmarshal(body["usage"], &usage); err != nil || len(usage) != 1 {
				t.Fatal("usage shape changed", err)
			}
			for _, field := range []string{"unit_cost", "total_amount"} {
				_, exposed := usage[0][field]
				if exposed != (role == auth.RoleAdmin) {
					t.Fatalf("%s visibility changed for %s", field, role)
				}
			}
			if role == auth.RoleAdmin && (string(usage[0]["unit_cost"]) != `"1.234567"` || string(usage[0]["total_amount"]) != `"12.34"`) {
				t.Fatal("exact decimal string contract changed")
			}
			if role == auth.RoleWorker {
				command, err := service.Complete(context.Background(), actor, id, Command{IdempotencyKey: id, ExpectedVersion: 3})
				if err != nil {
					t.Fatal(err)
				}
				commandJSON, err := json.Marshal(command)
				if err != nil || string(commandJSON) != string(data) {
					t.Fatal("legacy command must return full role-safe state, not a receipt", err)
				}
			}
		})
	}
}

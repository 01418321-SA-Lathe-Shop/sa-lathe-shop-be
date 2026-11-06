package delivery

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
)

type contractRepository struct {
	Repository
	state State
}

func (r *contractRepository) FulfillmentState(context.Context, string, string) (State, error) {
	return r.state, nil
}
func (r *contractRepository) FulfillmentJobs(context.Context, string, int, int) ([]State, error) {
	return []State{r.state}, nil
}
func (r *contractRepository) ApplyFulfillment(context.Context, string, string, Action) (State, error) {
	return r.state, nil
}

func TestLegacyFulfillmentResponseContract(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000001"
	days := 14
	instant := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, role := range []string{auth.RoleAdmin, auth.RoleWorker, auth.RoleCustomer} {
		t.Run(role, func(t *testing.T) {
			repository := &contractRepository{state: State{
				JobID: id, RequestID: id, JobStatus: "completed", RequestStatus: "ongoing", Version: 4, Cycle: 1, CreditDays: &days,
				OpeningBalance: money("100.00"), Deposit: money("50.00"),
				Invoice:    &Invoice{ID: id, Number: "INV-1", Amount: money("100.00"), Paid: money("25.00"), Balance: money("75.00"), Status: "partial", CreditDays: days, IssuedAt: instant, DueAt: instant.Add(14 * 24 * time.Hour)},
				Deliveries: []Shipment{}, Events: []Event{}, ReworkHistory: []ReworkCycle{}, Photos: []JobPhoto{},
				Payments: []Payment{{ID: id, Method: "bank", Amount: money("25.00"), EvidenceID: ptrContract(id), PaidAt: instant}},
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
			want := []string{"job_id", "request_id", "job_status", "request_status", "version", "cycle", "credit_days", "opening_balance", "deposit", "invoice", "deliveries", "events", "rework_history", "payments", "photos"}
			keys := make([]string, 0, len(body))
			for key := range body {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			sort.Strings(want)
			if !reflect.DeepEqual(keys, want) {
				t.Fatalf("legacy keys changed: got %v want %v", keys, want)
			}
			for _, field := range []string{"deliveries", "events", "rework_history", "photos"} {
				if string(body[field]) != "[]" {
					t.Fatalf("%s must remain an empty array", field)
				}
			}
			var invoice map[string]json.RawMessage
			if err = json.Unmarshal(body["invoice"], &invoice); err != nil {
				t.Fatal(err)
			}
			if string(invoice["balance"]) != `"75.00"` || string(body["deposit"]) != `"50.00"` {
				t.Fatal("decimal string contract changed")
			}
			list, err := service.List(context.Background(), actor, 50, 0)
			if err != nil || len(list) != 1 {
				t.Fatal("legacy list changed", err)
			}
			listJSON, err := json.Marshal(list[0])
			if err != nil || string(listJSON) != string(data) {
				t.Fatal("list/detail role redaction differs", err)
			}
			if role != auth.RoleCustomer {
				result, err := service.Issue(context.Background(), actor, id, Command{IdempotencyKey: id, ExpectedVersion: 3})
				if err != nil {
					t.Fatal(err)
				}
				commandJSON, err := json.Marshal(result)
				if err != nil || string(commandJSON) != string(data) {
					t.Fatal("legacy command must return full role-safe state", err)
				}
			}
		})
	}
}

func ptrContract(s string) *string { return &s }

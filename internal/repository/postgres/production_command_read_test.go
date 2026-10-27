package postgres

import (
	"encoding/json"
	"strings"
	"testing"

	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
)

func TestWorkerProductionSnapshotRetainsLegacyHistoryWithoutCosts(t *testing.T) {
	cost, err := m.ParseDecimal("12.345678")
	if err != nil {
		t.Fatal(err)
	}
	amount, err := q.ParseAmount("12.34")
	if err != nil {
		t.Fatal(err)
	}
	value := workerProductionSnapshot(p.State{Summary: p.Summary{Version: 7}, Finance: &p.Finance{MaterialCost: amount}, Usage: []p.Usage{{ID: "issue", UnitCost: &cost, Total: &amount}}, TimeEntries: []p.TimeEntry{{ID: "time"}}, Events: []p.Event{{ID: "event"}}, Photos: []p.Photo{{ID: "photo"}}})
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{`"finance"`, `"unit_cost"`, `"total_amount"`} {
		if strings.Contains(string(data), private) {
			t.Fatal("worker snapshot leaked cost", private)
		}
	}
	if value.Version != 7 || len(value.Usage) != 1 || len(value.TimeEntries) != 1 || len(value.Events) != 1 || len(value.Photos) != 1 {
		t.Fatal("legacy history was truncated")
	}
}

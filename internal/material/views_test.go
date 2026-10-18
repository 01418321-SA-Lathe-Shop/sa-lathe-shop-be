package material

import (
	"encoding/json"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"strings"
	"testing"
)

func TestWorkerFinancialViews(t *testing.T) {
	a := auth.Account{Role: auth.RoleWorker}
	for _, v := range []any{MaterialViews(a, []Material{{}}), MovementViews(a, []Movement{{}})} {
		data, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		for _, field := range []string{"cost", "reason", "payload"} {
			if strings.Contains(string(data), field) {
				t.Fatal("worker finance leak", string(data))
			}
		}
	}
	data, _ := json.Marshal(MaterialViews(auth.Account{Role: auth.RoleAdmin}, []Material{{}}))
	if !strings.Contains(string(data), "avg_cost") {
		t.Fatal("Admin lost costs")
	}
}

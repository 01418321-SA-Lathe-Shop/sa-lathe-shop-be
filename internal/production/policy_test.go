package production

import "testing"

func TestLifecyclePolicy(t *testing.T) {
	state := State{Summary: Summary{Status: "queued", PlanVersion: 1}}
	start := Action{Kind: "start", PlanVersion: 1}
	if _, e := NextStatus(state, start); e == nil {
		t.Fatal("empty start")
	}
	state.Materials = []MaterialState{{Required: qty("5"), Reserved: qty("4")}}
	if _, e := NextStatus(state, start); e == nil {
		t.Fatal("partial start")
	}
	state.Materials[0].Reserved = qty("5")
	next, e := NextStatus(state, start)
	if e != nil || next != "in_progress" {
		t.Fatal(next, e)
	}
	state.Status = "completed"
	if _, e = NextStatus(state, Action{Kind: "use"}); e == nil {
		t.Fatal("closed usage")
	}
	if _, e = NextStatus(state, Action{Kind: "labor"}); e != nil {
		t.Fatal("post-completion actual cost", e)
	}
}

package dashboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
)

func TestCalendarWindows(t *testing.T) {
	now := time.Date(2024, 2, 28, 17, 0, 0, 0, time.UTC) // Feb 29 in Bangkok
	for _, tc := range []struct{ value, from string }{{"today", "2024-02-29"}, {"7d", "2024-02-23"}, {"30d", "2024-01-31"}, {"year", "2024-01-01"}, {"", "2024-01-31"}} {
		w, e := NewWindow(tc.value, now)
		if e != nil || w.From.Format("2006-01-02") != tc.from || w.HeatmapFrom.Format("2006-01-02") != "2023-03-01" || !w.AsOf.Equal(now) {
			t.Fatalf("%s: %+v %v", tc.value, w, e)
		}
	}
	if _, e := NewWindow("all", now); !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal(e)
	}
	w, _ := NewWindow("7d", time.Date(2026, 1, 1, 0, 0, 0, 0, bangkok))
	if w.From.Format("2006-01-02") != "2025-12-26" {
		t.Fatal(w)
	}
}

func TestHeatmapSummary(t *testing.T) {
	w, _ := NewWindow("30d", time.Date(2026, 10, 28, 12, 0, 0, 0, bangkok))
	h := CompleteHeatmap(w, []Day{{"2026-10-25", 2}, {"2026-10-26", 3}, {"2026-10-27", 3}}, 0)
	if len(h.Days) != 365 || h.Summary.Total != 8 || *h.Summary.BestDay != "2026-10-27" || *h.Summary.CurrentStreak != 3 || *h.Summary.AveragePerWeek != "0.2" {
		t.Fatal(h)
	}
	h = CompleteHeatmap(w, nil, 0)
	if h.Summary.BestDay != nil || *h.Summary.CurrentStreak != 0 || *h.Summary.AveragePerWeek != "0.0" {
		t.Fatal(h)
	}
	h = CompleteHeatmap(w, []Day{{"2026-10-28", 2}}, 1)
	if h.HistoryComplete || h.Summary.CurrentStreak != nil || h.Summary.AveragePerWeek != nil || h.Summary.Total != 2 {
		t.Fatal(h)
	}
}

type stub struct{ seen Filter }

func (r *stub) DashboardOverview(context.Context, string, Window) (Overview, error) {
	return Overview{}, nil
}
func (r *stub) DashboardJobs(_ context.Context, _ string, f Filter) (Jobs, error) {
	r.seen = f
	return Jobs{}, nil
}

func TestPermissionsAndFilters(t *testing.T) {
	r := &stub{}
	s := NewService(r)
	s.now = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, bangkok) }
	for _, a := range []auth.Account{{}, {Active: true, Role: auth.RoleCustomer}, {Active: false, Role: auth.RoleWorker}} {
		if _, e := s.Overview(context.Background(), a, ""); !errors.Is(e, req.ErrForbidden) {
			t.Fatal(e)
		}
		if _, e := s.Jobs(context.Background(), a, "", "", "", 20, 0); !errors.Is(e, req.ErrForbidden) {
			t.Fatal(e)
		}
	}
	for _, role := range []string{auth.RoleAdmin, auth.RoleWorker} {
		a := auth.Account{Active: true, Role: role}
		if _, e := s.Overview(context.Background(), a, "30d"); e != nil {
			t.Fatal(e)
		}
		v, e := s.Jobs(context.Background(), a, "", "", "2026-10-28", 20, 0)
		if e != nil || v.Items == nil || r.seen.FirstCompletedOn != "2026-10-28" {
			t.Fatal(v, e)
		}
		for _, f := range []Filter{{Status: "canceled"}, {FirstCompletedOn: "2026-02-30"}, {FirstCompletedOn: "2026-10-29"}, {Status: "queued", FirstCompletedOn: "2026-10-28"}, {Window: Window{Range: "30d"}, FirstCompletedOn: "2026-10-28"}} {
			if _, e := s.Jobs(context.Background(), a, f.Range, f.Status, f.FirstCompletedOn, 20, 0); !errors.Is(e, req.ErrInvalidInput) {
				t.Fatalf("%+v: %v", f, e)
			}
		}
	}
}

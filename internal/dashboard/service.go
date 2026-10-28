package dashboard

import (
	"context"
	"math/big"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
)

var bangkok = time.FixedZone("Asia/Bangkok", 7*60*60)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }
func staff(a auth.Account) bool {
	return a.Active && (a.Role == auth.RoleWorker || a.Role == auth.RoleAdmin)
}

// Periods select the creation cohort, not a historical status snapshot.
func NewWindow(value string, now time.Time) (Window, error) {
	if value == "" {
		value = "30d"
	}
	local := now.In(bangkok)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, bangkok)
	from := today
	switch value {
	case "today":
	case "7d":
		from = today.AddDate(0, 0, -6)
	case "30d":
		from = today.AddDate(0, 0, -29)
	case "year":
		from = time.Date(local.Year(), 1, 1, 0, 0, 0, 0, bangkok)
	default:
		return Window{}, req.ErrInvalidInput
	}
	// Clamp leap day before taking the day after the prior-year boundary.
	previous := time.Date(local.Year()-1, local.Month(), local.Day(), 0, 0, 0, 0, bangkok)
	if previous.Month() != local.Month() {
		previous = time.Date(local.Year()-1, local.Month()+1, 0, 0, 0, 0, 0, bangkok)
	}
	return Window{value, from, now.UTC(), previous.AddDate(0, 0, 1), today}, nil
}

func (s *Service) Overview(ctx context.Context, a auth.Account, value string) (Overview, error) {
	if !staff(a) {
		return Overview{}, req.ErrForbidden
	}
	w, err := NewWindow(value, s.now())
	if err != nil {
		return Overview{}, err
	}
	v, err := s.repo.DashboardOverview(ctx, a.ID, w)
	if err != nil {
		return Overview{}, err
	}
	v.AsOf, v.Range, v.Period = w.AsOf, w.Range, Period{w.From, w.AsOf}
	v.Heatmap = CompleteHeatmap(w, v.Heatmap.Days, v.Heatmap.MissingHistoryJobs)
	return v, nil
}

func (s *Service) Jobs(ctx context.Context, a auth.Account, value, status, date string, limit, offset int) (Jobs, error) {
	if !staff(a) {
		return Jobs{}, req.ErrForbidden
	}
	w, err := NewWindow(value, s.now())
	if err != nil || limit < 1 || limit > 100 || offset < 0 {
		return Jobs{}, req.ErrInvalidInput
	}
	if status != "" && status != "queued" && status != "in_progress" && status != "completed" && status != "cancelled" {
		return Jobs{}, req.ErrInvalidInput
	}
	if date != "" {
		day, e := time.ParseInLocation("2006-01-02", date, bangkok)
		if e != nil || date != day.Format("2006-01-02") || day.Before(w.HeatmapFrom) || day.After(w.Today) || value != "" || status != "" {
			return Jobs{}, req.ErrInvalidInput
		}
	}
	v, err := s.repo.DashboardJobs(ctx, a.ID, Filter{w, status, date, limit, offset})
	v.AsOf, v.Limit, v.Offset = w.AsOf, limit, offset
	if v.Items == nil {
		v.Items = []Job{}
	}
	return v, err
}

// CompleteHeatmap fills only known calendar days, never inventing events.
// Unknown legacy history suppresses streak/average rather than asserting zeros.
func CompleteHeatmap(w Window, sparse []Day, missing int64) Heatmap {
	counts := make(map[string]int64, len(sparse))
	for _, day := range sparse {
		counts[day.Date] = day.Count
	}
	h := Heatmap{StartDate: w.HeatmapFrom.Format("2006-01-02"), EndDate: w.Today.Format("2006-01-02"), Days: []Day{}, HistoryComplete: missing == 0, MissingHistoryJobs: missing}
	for d := w.HeatmapFrom; !d.After(w.Today); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		count := counts[date]
		h.Days = append(h.Days, Day{date, count})
		h.Summary.Total += count
		if count > 0 && count >= h.Summary.BestDayCount {
			chosen := date
			h.Summary.BestDay, h.Summary.BestDayCount = &chosen, count
		}
	}
	if h.HistoryComplete {
		var streak int64
		i := len(h.Days) - 1
		if h.Days[i].Count == 0 {
			i--
		}
		for ; i >= 0 && h.Days[i].Count > 0; i-- {
			streak++
		}
		average := new(big.Rat).SetInt64(h.Summary.Total)
		average.Mul(average, big.NewRat(7, int64(len(h.Days))))
		formatted := average.FloatString(1)
		h.Summary.CurrentStreak, h.Summary.AveragePerWeek = &streak, &formatted
	}
	return h
}

package request

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const accountID = "57039d8b-c58e-4ee1-8943-539dc6203626"

type fakeRepository struct {
	calls     int
	result    Request
	err       error
	owner, id string
}

func (f *fakeRepository) Create(_ context.Context, r Request) (Request, error) {
	f.calls++
	f.result = r
	return r, f.err
}
func (f *fakeRepository) GetOwned(_ context.Context, owner, id string) (Request, error) {
	f.calls++
	f.owner = owner
	f.id = id
	return f.result, f.err
}
func (f *fakeRepository) DeletePending(_ context.Context, owner, id string) error {
	f.calls++
	f.owner = owner
	f.id = id
	return f.err
}
func (f *fakeRepository) RequestMoreInfo(_ context.Context, _, id, note string) (Request, error) {
	f.calls++
	f.id = id
	f.result.InfoRequestNote = note
	return f.result, f.err
}
func customer() Principal { return Principal{AccountID: accountID, Role: "customer", Active: true} }
func (f *fakeRepository) ListRequests(_ context.Context, _ string, _, _ int) ([]Request, error) {
	f.calls++
	return []Request{f.result}, f.err
}

func TestSubmit(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	deadline := now.Add(24 * time.Hour).In(time.FixedZone("Bangkok", 7*3600))
	got, err := service.Submit(context.Background(), customer(), Input{CustomerID: accountID, Description: "  แม่พิมพ์อัดยาง  ", Quantity: 1, MaterialSpec: " S45C ", Deadline: &deadline})
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "แม่พิมพ์อัดยาง" || got.MaterialSpec != "S45C" || got.CreatedBy != accountID || got.Source != "net" || got.Status != "pending" || !validID(got.ID) || !got.CreatedAt.Equal(now) || got.Deadline.Location() != time.UTC {
		t.Fatalf("unexpected request: %+v", got)
	}
	if deadline.Location() == time.UTC {
		t.Fatal("mutated caller deadline")
	}
}

func TestSubmitValidation(t *testing.T) {
	for name, input := range map[string]Input{
		"empty":                {CustomerID: accountID, Description: " ", Quantity: 1},
		"zero":                 {CustomerID: accountID, Description: "job", Quantity: 0},
		"negative":             {CustomerID: accountID, Description: "job", Quantity: -1},
		"description too long": {CustomerID: accountID, Description: strings.Repeat("ก", MaxDescriptionCharacters+1), Quantity: 1},
		"material too long":    {CustomerID: accountID, Description: "job", Quantity: 1, MaterialSpec: strings.Repeat("a", MaxMaterialSpecCharacters+1)},
		"invalid utf8":         {CustomerID: accountID, Description: string([]byte{0xff}), Quantity: 1},
		"NUL description":      {CustomerID: accountID, Description: "job\x00name", Quantity: 1},
		"NUL material":         {CustomerID: accountID, Description: "job", Quantity: 1, MaterialSpec: "S45\x00C"},
		"past deadline":        {CustomerID: accountID, Description: "job", Quantity: 1, Deadline: func() *time.Time { v := time.Unix(1, 0); return &v }()},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepository{}
			_, err := NewService(repo).Submit(context.Background(), customer(), input)
			if !errors.Is(err, ErrInvalidInput) || repo.calls != 0 {
				t.Fatalf("err=%v calls=%d", err, repo.calls)
			}
		})
	}
}

func TestRejectNonCustomerBeforeRepository(t *testing.T) {
	for _, p := range []Principal{{}, {AccountID: accountID, Role: "worker", Active: true}, {AccountID: accountID, Role: "admin", Active: true}, {AccountID: accountID, Role: "customer"}, {AccountID: "bad", Role: "customer", Active: true}} {
		repo := &fakeRepository{}
		service := NewService(repo)
		_, err := service.Submit(context.Background(), p, Input{CustomerID: accountID, Description: "job", Quantity: 1})
		if !errors.Is(err, ErrForbidden) {
			t.Fatal(err)
		}
		if repo.calls != 0 {
			t.Fatalf("err=%v calls=%d", err, repo.calls)
		}
	}
}

func TestGetOwnershipAndNotFound(t *testing.T) {
	repo := &fakeRepository{err: ErrNotFound}
	_, err := NewService(repo).Get(context.Background(), customer(), accountID)
	if !errors.Is(err, ErrNotFound) || repo.owner != accountID || repo.id != accountID {
		t.Fatalf("ownership not passed: %+v %v", repo, err)
	}
}

func TestRepositoryAndCancellationErrors(t *testing.T) {
	sentinel := errors.New("database unavailable")
	repo := &fakeRepository{err: sentinel}
	service := NewService(repo)
	_, err := service.Submit(context.Background(), customer(), Input{CustomerID: accountID, Description: "job", Quantity: 1})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo.calls = 0
	_, err = service.Submit(ctx, customer(), Input{CustomerID: accountID, Description: "job", Quantity: 1})
	if !errors.Is(err, context.Canceled) || repo.calls != 0 {
		t.Fatal("canceled submit touched repository")
	}
	_, err = service.Get(ctx, customer(), accountID)
	if !errors.Is(err, context.Canceled) || repo.calls != 0 {
		t.Fatal("canceled get touched repository")
	}
}

type resubmitRepository struct {
	fakeRepository
	got Correction
}

func (f *resubmitRepository) Resubmit(_ context.Context, _, _ string, in Correction) (Request, error) {
	f.calls++
	f.got = in
	return Request{Input: Input{Description: in.Description}}, nil
}

func TestResubmitValidatesCorrection(t *testing.T) {
	repo := &resubmitRepository{}
	s := NewService(repo)
	customer := Principal{AccountID: accountID, Role: "customer", Active: true}
	id := "8a7e3e8b-2f3d-4c55-9f8f-1d3b2f3a9c11"
	past := time.Now().Add(-time.Hour)
	for _, bad := range []Correction{{Description: " ", Quantity: 1}, {Description: "x", Quantity: 0}, {Description: "x", Quantity: 1, MaterialSpec: strings.Repeat("a", MaxMaterialSpecCharacters+1)}, {Description: "x", Quantity: 1, Deadline: &past}} {
		if _, err := s.Resubmit(context.Background(), customer, id, bad); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid correction accepted: %+v", bad)
		}
	}
	if _, err := s.Resubmit(context.Background(), Principal{AccountID: accountID, Role: "worker", Active: true}, id, Correction{Description: "x", Quantity: 1}); !errors.Is(err, ErrForbidden) {
		t.Fatal("worker must not edit customer request content")
	}
	if repo.calls != 0 {
		t.Fatal("invalid correction reached the repository")
	}
	if _, err := s.Resubmit(context.Background(), customer, id, Correction{Description: "  fixed  ", Quantity: 2}); err != nil || repo.got.Description != "fixed" {
		t.Fatal("valid correction", err, repo.got)
	}
}

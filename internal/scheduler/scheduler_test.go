package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMonthlyScheduleLatestDueUsesPreviousTrigger(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := NewMonthlySchedule(loc, 3, 0)
	if err != nil {
		t.Fatalf("NewMonthlySchedule() error = %v", err)
	}

	beforeCurrentTrigger := time.Date(2026, time.August, 1, 2, 30, 0, 0, loc)
	got, ok := schedule.LatestDue(beforeCurrentTrigger)
	if !ok {
		t.Fatal("LatestDue() ok = false, want true")
	}
	want := time.Date(2026, time.July, 1, 3, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("LatestDue() = %v, want %v", got, want)
	}

	afterCurrentTrigger := time.Date(2026, time.August, 17, 19, 30, 0, 0, loc)
	got, ok = schedule.LatestDue(afterCurrentTrigger)
	if !ok {
		t.Fatal("LatestDue() after trigger ok = false, want true")
	}
	want = time.Date(2026, time.August, 1, 3, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("LatestDue() after trigger = %v, want %v", got, want)
	}
}

func TestCatchUpLatestOnlyIsIdempotent(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := NewMonthlySchedule(loc, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 17, 19, 30, 0, 0, loc)
	store := newFakeRunStore()
	s, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var runs []time.Time
	job := Job{
		HouseholdID: 7,
		Name:        "monthly_report",
		Schedule:    schedule,
		CatchUp:     CatchUpLatestOnly,
		Run: func(_ context.Context, scheduledFor time.Time) error {
			runs = append(runs, scheduledFor)
			return nil
		},
	}

	if err := s.CatchUp(context.Background(), job); err != nil {
		t.Fatalf("CatchUp() error = %v", err)
	}
	if err := s.CatchUp(context.Background(), job); err != nil {
		t.Fatalf("second CatchUp() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("job runs = %d, want 1", len(runs))
	}
	wantScheduledFor := time.Date(2026, time.August, 1, 3, 0, 0, 0, loc)
	if !runs[0].Equal(wantScheduledFor) {
		t.Fatalf("scheduled_for = %v, want %v", runs[0], wantScheduledFor)
	}
	key := RunKey{HouseholdID: 7, JobName: "monthly_report", ScheduledFor: wantScheduledFor}
	if got := store.outcomes[key]; got.Status != RunSucceeded || got.ErrorCode != "" {
		t.Fatalf("outcome = %#v, want succeeded", got)
	}
}

func TestRunDuePersistsFailureWithoutRawErrorText(t *testing.T) {
	jobErr := errors.New("provider returned secret-bearing diagnostic")
	store := newFakeRunStore()
	now := time.Date(2026, time.August, 1, 3, 0, 1, 0, time.UTC)
	s, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	key := RunKey{HouseholdID: 3, JobName: "monthly_report", ScheduledFor: now.Add(-time.Second)}
	job := Job{
		HouseholdID: key.HouseholdID,
		Name:        key.JobName,
		Run: func(context.Context, time.Time) error {
			return jobErr
		},
	}

	err = s.RunDue(context.Background(), job, key.ScheduledFor)
	if !errors.Is(err, jobErr) {
		t.Fatalf("RunDue() error = %v, want wrapped job error", err)
	}
	outcome := store.outcomes[key]
	if outcome.Status != RunFailed || outcome.ErrorCode != ErrorCodeJobFailed {
		t.Fatalf("outcome = %#v, want failed/%q", outcome, ErrorCodeJobFailed)
	}
	if outcome.ErrorCode == jobErr.Error() {
		t.Fatal("raw job error text leaked into persisted error code")
	}
}

func TestRunDuePersistsPeriodDerivedFromScheduledTrigger(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 1, 3, 0, 1, 0, loc)
	store := newFakeRunStore()
	s, err := New(store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	scheduledFor := time.Date(2026, time.August, 1, 3, 0, 0, 0, loc)
	job := Job{
		HouseholdID: 9,
		Name:        "monthly_report",
		Period: func(trigger time.Time) string {
			return trigger.In(loc).AddDate(0, -1, 0).Format("2006-01")
		},
		Run: func(context.Context, time.Time) error { return nil },
	}

	if err := s.RunDue(context.Background(), job, scheduledFor); err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	key := RunKey{HouseholdID: 9, JobName: "monthly_report", ScheduledFor: scheduledFor, Period: "2026-07"}
	if !store.claimed[key] {
		t.Fatalf("derived key %#v was not claimed", key)
	}
	if got := store.outcomes[key]; got.Status != RunSucceeded {
		t.Fatalf("outcome = %#v, want succeeded", got)
	}
}

func TestRunReturnsOnCancelledContext(t *testing.T) {
	loc := time.UTC
	schedule, err := NewMonthlySchedule(loc, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 17, 19, 30, 0, 0, loc)
	s, err := New(newFakeRunStore(), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	job := Job{
		HouseholdID: 1,
		Name:        "monthly_report",
		Schedule:    schedule,
		CatchUp:     CatchUpNone,
		Run:         func(context.Context, time.Time) error { return nil },
	}

	if err := s.Run(ctx, job); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

type fakeRunStore struct {
	claimed  map[RunKey]bool
	outcomes map[RunKey]RunOutcome
}

func newFakeRunStore() *fakeRunStore {
	return &fakeRunStore{
		claimed:  make(map[RunKey]bool),
		outcomes: make(map[RunKey]RunOutcome),
	}
}

func (s *fakeRunStore) Claim(_ context.Context, key RunKey, _ time.Time) (bool, error) {
	if s.claimed[key] {
		return false, nil
	}
	s.claimed[key] = true
	return true, nil
}

func (s *fakeRunStore) Finish(_ context.Context, key RunKey, _ time.Time, outcome RunOutcome) error {
	s.outcomes[key] = outcome
	return nil
}

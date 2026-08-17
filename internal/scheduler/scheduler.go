package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidScheduler = errors.New("invalid scheduler")
	ErrInvalidSchedule  = errors.New("invalid schedule")
	ErrInvalidJob       = errors.New("invalid job")
)

const ErrorCodeJobFailed = "job_failed"

type RunStatus string

const (
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
)

type RunKey struct {
	HouseholdID  int64
	JobName      string
	ScheduledFor time.Time
	Period       string
}

type RunOutcome struct {
	Status    RunStatus
	ErrorCode string
}

type RunRecord struct {
	ID         int64
	Key        RunKey
	Status     RunStatus
	StartedAt  time.Time
	FinishedAt time.Time
	ErrorCode  string
}

type HouseholdScope struct {
	HouseholdID int64
	Timezone    string
}

type RunStore interface {
	Claim(ctx context.Context, key RunKey, startedAt time.Time) (bool, error)
	Finish(ctx context.Context, key RunKey, finishedAt time.Time, outcome RunOutcome) error
}

type CatchUpPolicy uint8

const (
	CatchUpNone CatchUpPolicy = iota
	CatchUpLatestOnly
)

type Schedule interface {
	LatestDue(at time.Time) (time.Time, bool)
	Next(after time.Time) time.Time
}

type MonthlySchedule struct {
	location *time.Location
	hour     int
	minute   int
}

func NewMonthlySchedule(location *time.Location, hour, minute int) (MonthlySchedule, error) {
	if location == nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return MonthlySchedule{}, ErrInvalidSchedule
	}
	return MonthlySchedule{location: location, hour: hour, minute: minute}, nil
}

func (s MonthlySchedule) LatestDue(at time.Time) (time.Time, bool) {
	local := at.In(s.location)
	candidate := time.Date(local.Year(), local.Month(), 1, s.hour, s.minute, 0, 0, s.location)
	if candidate.After(local) {
		previous := candidate.AddDate(0, -1, 0)
		return previous, true
	}
	return candidate, true
}

func (s MonthlySchedule) Next(after time.Time) time.Time {
	local := after.In(s.location)
	candidate := time.Date(local.Year(), local.Month(), 1, s.hour, s.minute, 0, 0, s.location)
	if !candidate.After(local) {
		candidate = candidate.AddDate(0, 1, 0)
	}
	return candidate
}

type Scheduler struct {
	store RunStore
	now   func() time.Time
}

func New(store RunStore, now func() time.Time) (*Scheduler, error) {
	if store == nil {
		return nil, ErrInvalidScheduler
	}
	if now == nil {
		now = time.Now
	}
	return &Scheduler{store: store, now: now}, nil
}

type Job struct {
	HouseholdID int64
	Name        string
	Schedule    Schedule
	CatchUp     CatchUpPolicy
	Period      func(time.Time) string
	Run         func(context.Context, time.Time) error
}

func (s *Scheduler) CatchUp(ctx context.Context, job Job) error {
	if err := validateJob(job, true); err != nil {
		return err
	}
	switch job.CatchUp {
	case CatchUpNone:
		return nil
	case CatchUpLatestOnly:
		due, ok := job.Schedule.LatestDue(s.now())
		if !ok {
			return nil
		}
		return s.RunDue(ctx, job, due)
	default:
		return fmt.Errorf("%w: unsupported catch-up policy", ErrInvalidJob)
	}
}

func (s *Scheduler) RunDue(ctx context.Context, job Job, scheduledFor time.Time) error {
	if err := validateJob(job, false); err != nil {
		return err
	}
	period := ""
	if job.Period != nil {
		period = strings.TrimSpace(job.Period(scheduledFor))
	}
	key := RunKey{
		HouseholdID:  job.HouseholdID,
		JobName:      strings.TrimSpace(job.Name),
		ScheduledFor: scheduledFor,
		Period:       period,
	}
	claimed, err := s.store.Claim(ctx, key, s.now())
	if err != nil {
		return fmt.Errorf("claim job run: %w", err)
	}
	if !claimed {
		return nil
	}

	runErr := job.Run(ctx, scheduledFor)
	outcome := RunOutcome{Status: RunSucceeded}
	if runErr != nil {
		outcome = RunOutcome{Status: RunFailed, ErrorCode: ErrorCodeJobFailed}
	}
	finishErr := s.store.Finish(ctx, key, s.now(), outcome)
	if finishErr != nil {
		if runErr != nil {
			return errors.Join(runErr, fmt.Errorf("finish job run: %w", finishErr))
		}
		return fmt.Errorf("finish job run: %w", finishErr)
	}
	if runErr != nil {
		return fmt.Errorf("run job %q: %w", key.JobName, runErr)
	}
	return nil
}

func (s *Scheduler) Run(ctx context.Context, job Job) error {
	if err := validateJob(job, true); err != nil {
		return err
	}
	if err := s.CatchUp(ctx, job); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		now := s.now()
		next := job.Schedule.Next(now)
		delay := next.Sub(now)
		if delay < 0 {
			delay = 0
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case scheduledFor := <-timer.C:
			if err := s.RunDue(ctx, job, scheduledFor); err != nil {
				return err
			}
		}
	}
}

func validateJob(job Job, requireSchedule bool) error {
	if job.HouseholdID <= 0 || strings.TrimSpace(job.Name) == "" || job.Run == nil {
		return ErrInvalidJob
	}
	if requireSchedule && job.Schedule == nil {
		return ErrInvalidJob
	}
	return nil
}

package process

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidEvent = errors.New("invalid process event")
	ErrNotStarted   = errors.New("process is not started")
	ErrNotReady     = errors.New("process is not ready for event")
)

type Policy struct {
	StepTimeout time.Duration
	MaxAttempts int
}

type Transition[Snapshot, Command any] struct {
	Snapshot Snapshot
	Command  *Command
	Changed  bool
	Terminal bool
}

type Definition[Snapshot, Event, Command any] interface {
	Policy() Policy
	Apply(current *Snapshot, event Event) (Transition[Snapshot, Command], error)
	OnTimeout(current Snapshot, attempt int) (Transition[Snapshot, Command], error)
}

func Invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidEvent, fmt.Sprintf(format, args...))
}

func NotStarted(instanceID string) error {
	return fmt.Errorf("%w: %s", ErrNotStarted, instanceID)
}

func NotReady(instanceID, event string) error {
	return fmt.Errorf("%w: %s cannot accept %s", ErrNotReady, instanceID, event)
}

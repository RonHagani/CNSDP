package main

import "fmt"

// EnvironmentError indicates the harness itself could not execute
// correctly -- Docker/Compose unavailable, a container failed to start,
// the database was unreachable, a subprocess timed out -- as distinct from
// a genuine product/AC requirement failure (a measured threshold
// violated). This distinction drives the exit-code taxonomy in main.go: an
// environment error must never be reported or exit-coded the same way as
// "the product failed to meet an approved requirement," since the former
// says nothing about the product under test.
type EnvironmentError struct {
	Op  string
	Err error
}

func (e *EnvironmentError) Error() string {
	return fmt.Sprintf("environment error during %s: %v", e.Op, e.Err)
}

func (e *EnvironmentError) Unwrap() error { return e.Err }

func envErrorf(op string, format string, args ...any) *EnvironmentError {
	return &EnvironmentError{Op: op, Err: fmt.Errorf(format, args...)}
}

func wrapEnvErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return &EnvironmentError{Op: op, Err: err}
}

// Exit codes, checked by main.go and documented in the human-readable
// report footer.
const (
	exitOK                = 0 // full pass (full profile), or smoke completed cleanly
	exitRequirementFailed = 1 // full profile only: at least one AC verdict is FAIL or NOT_IMPLEMENTED
	exitEnvironmentError  = 2 // harness could not execute correctly; no AC verdict is authoritative
)

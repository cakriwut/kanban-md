package task

import (
	"errors"
	"testing"

	"github.com/antopolskiy/kanban-md/internal/clierr"
)

func TestValidateStatus_Valid(t *testing.T) {
	err := ValidateStatus("todo", []string{"backlog", "todo", "done"})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateStatus_Invalid(t *testing.T) {
	err := ValidateStatus("invalid", []string{"backlog", "todo", "done"})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	var cliErr *clierr.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected clierr.Error, got %T", err)
	}
	if cliErr.Code != clierr.InvalidStatus {
		t.Errorf("code = %q, want %q", cliErr.Code, clierr.InvalidStatus)
	}
	if cliErr.Details["status"] != "invalid" {
		t.Errorf("details[status] = %v, want %q", cliErr.Details["status"], "invalid")
	}
}

func TestValidatePriority_Valid(t *testing.T) {
	err := ValidatePriority("high", []string{"low", "medium", "high"})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidatePriority_Invalid(t *testing.T) {
	err := ValidatePriority("urgent", []string{"low", "medium", "high"})
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}
	var cliErr *clierr.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected clierr.Error, got %T", err)
	}
	if cliErr.Code != clierr.InvalidPriority {
		t.Errorf("code = %q, want %q", cliErr.Code, clierr.InvalidPriority)
	}
	if cliErr.Details["priority"] != "urgent" {
		t.Errorf("details[priority] = %v, want %q", cliErr.Details["priority"], "urgent")
	}
}

func TestValidateDate(t *testing.T) {
	err := ValidateDate("started", "not-a-date", errors.New("parse error"))
	if err.Code != clierr.InvalidDate {
		t.Errorf("code = %q, want %q", err.Code, clierr.InvalidDate)
	}
	if err.Details["field"] != "started" {
		t.Errorf("details[field] = %v, want %q", err.Details["field"], "started")
	}
	if err.Details["input"] != "not-a-date" {
		t.Errorf("details[input] = %v, want %q", err.Details["input"], "not-a-date")
	}
}

func TestValidateTaskID(t *testing.T) {
	err := ValidateTaskID("abc")
	if err.Code != clierr.InvalidTaskID {
		t.Errorf("code = %q, want %q", err.Code, clierr.InvalidTaskID)
	}
	if err.Details["input"] != "abc" {
		t.Errorf("details[input] = %v, want %q", err.Details["input"], "abc")
	}
}

func TestValidateSelfReference(t *testing.T) {
	err := ValidateSelfReference(42)
	if err.Code != clierr.SelfReference {
		t.Errorf("code = %q, want %q", err.Code, clierr.SelfReference)
	}
	if err.Details["id"] != 42 {
		t.Errorf("details[id] = %v, want 42", err.Details["id"])
	}
}

func TestValidateDependencyNotFound(t *testing.T) {
	err := ValidateDependencyNotFound(99)
	if err.Code != clierr.DependencyNotFound {
		t.Errorf("code = %q, want %q", err.Code, clierr.DependencyNotFound)
	}
	if err.Details["id"] != 99 {
		t.Errorf("details[id] = %v, want 99", err.Details["id"])
	}
}

func TestValidateWIPLimit(t *testing.T) {
	err := ValidateWIPLimit("in-progress", 3, 3)
	if err.Code != clierr.WIPLimitExceeded {
		t.Errorf("code = %q, want %q", err.Code, clierr.WIPLimitExceeded)
	}
	if err.Details["status"] != "in-progress" {
		t.Errorf("details[status] = %v, want %q", err.Details["status"], "in-progress")
	}
	if err.Details["limit"] != 3 {
		t.Errorf("details[limit] = %v, want 3", err.Details["limit"])
	}
	if err.Details["current"] != 3 {
		t.Errorf("details[current] = %v, want 3", err.Details["current"])
	}
}

func TestValidateBoundaryError(t *testing.T) {
	err := ValidateBoundaryError(5, "done", "last")
	if err.Code != clierr.BoundaryError {
		t.Errorf("code = %q, want %q", err.Code, clierr.BoundaryError)
	}
	if err.Details["id"] != 5 {
		t.Errorf("details[id] = %v, want 5", err.Details["id"])
	}
	if err.Details["direction"] != "last" {
		t.Errorf("details[direction] = %v, want %q", err.Details["direction"], "last")
	}
}

func TestFormatDueDate(t *testing.T) {
	err := FormatDueDate("2026-13-01", errors.New("month out of range"))
	if err.Code != clierr.InvalidDate {
		t.Errorf("code = %q, want %q", err.Code, clierr.InvalidDate)
	}
	if err.Details["field"] != "due" {
		t.Errorf("details[field] = %v, want %q", err.Details["field"], "due")
	}
}

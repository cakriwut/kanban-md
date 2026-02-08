package config

import (
	"errors"
	"testing"
)

func TestMigrateCurrentVersionNoop(t *testing.T) {
	cfg := NewDefault("Test")
	if err := migrate(cfg); err != nil {
		t.Errorf("migrate() current version: %v", err)
	}
	if cfg.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, CurrentVersion)
	}
}

func TestMigrateNewerVersionErrors(t *testing.T) {
	cfg := NewDefault("Test")
	cfg.Version = CurrentVersion + 1

	err := migrate(cfg)
	if err == nil {
		t.Fatal("migrate() newer version: expected error, got nil")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("migrate() error = %v, want ErrInvalid", err)
	}
}

func TestMigrateZeroVersionErrors(t *testing.T) {
	cfg := NewDefault("Test")
	cfg.Version = 0

	err := migrate(cfg)
	if err == nil {
		t.Fatal("migrate() version 0: expected error, got nil")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("migrate() error = %v, want ErrInvalid", err)
	}
}

func TestMigrateNegativeVersionErrors(t *testing.T) {
	cfg := NewDefault("Test")
	cfg.Version = -1

	err := migrate(cfg)
	if err == nil {
		t.Fatal("migrate() negative version: expected error, got nil")
	}
}

func TestMigrateV1ToCurrentVersion(t *testing.T) {
	cfg := NewDefault("Test")
	cfg.Version = 1
	// Clear fields that v2→v3 migration should set.
	cfg.ClaimTimeout = ""
	cfg.Classes = nil
	cfg.Defaults.Class = ""

	if err := migrate(cfg); err != nil {
		t.Fatalf("migrate() v1→current: %v", err)
	}
	if cfg.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, CurrentVersion)
	}
}

func TestMigrateV2ToV3(t *testing.T) {
	cfg := NewDefault("Test")
	cfg.Version = 2
	// Clear fields that v2→v3 migration should set.
	cfg.ClaimTimeout = ""
	cfg.Classes = nil
	cfg.Defaults.Class = ""

	if err := migrate(cfg); err != nil {
		t.Fatalf("migrate() v2→v3: %v", err)
	}
	if cfg.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, CurrentVersion)
	}
	if cfg.ClaimTimeout != DefaultClaimTimeout {
		t.Errorf("ClaimTimeout = %q, want %q", cfg.ClaimTimeout, DefaultClaimTimeout)
	}
	const expectedClasses = 4
	if len(cfg.Classes) != expectedClasses {
		t.Errorf("Classes len = %d, want %d", len(cfg.Classes), expectedClasses)
	}
	if cfg.Defaults.Class != DefaultClass {
		t.Errorf("Defaults.Class = %q, want %q", cfg.Defaults.Class, DefaultClass)
	}
}

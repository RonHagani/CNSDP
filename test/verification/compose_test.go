package main

import "testing"

// realGoodConfig is the actual `docker compose -f docker-compose.yml -f
// docker-compose.verify.yml -p cnsdp-verify config` output captured while
// designing docker-compose.verify.yml against this repository's real
// Docker Compose v2.40.3-desktop.1 -- not a hand-written fixture. It
// proves !override actually replaced the base port entry (only one `app`
// port publication, no `published` field) on the installed Compose
// version, and is pinned here as a regression guard.
const realGoodConfig = `
name: cnsdp-verify
services:
  app:
    ports:
      - mode: ingress
        host_ip: 127.0.0.1
        target: 8080
        protocol: tcp
  postgres:
    ports:
      - mode: ingress
        host_ip: 127.0.0.1
        target: 5432
        protocol: tcp
`

// brokenMergeConfig simulates what the rendered config would look like if
// Compose's default list-merge semantics applied instead of !override --
// the base file's fixed "127.0.0.1:8080:8080" entry surviving alongside
// the override's dynamic one. The detector must catch this.
const brokenMergeConfig = `
name: cnsdp-verify
services:
  app:
    ports:
      - mode: ingress
        host_ip: 127.0.0.1
        target: 8080
        published: "8080"
        protocol: tcp
      - mode: ingress
        host_ip: 127.0.0.1
        target: 8080
        protocol: tcp
  postgres:
    ports:
      - mode: ingress
        host_ip: 127.0.0.1
        target: 5432
        protocol: tcp
`

func TestValidateIsolationFromConfig_RealGoodConfig_Satisfied(t *testing.T) {
	report, err := validateIsolationFromConfig(realGoodConfig)
	if err != nil {
		t.Fatalf("validateIsolationFromConfig: %v", err)
	}
	if !report.Satisfied {
		t.Errorf("Satisfied = false, want true; problems: %v", report.Problems)
	}
	if report.AppPortCount != 1 {
		t.Errorf("AppPortCount = %d, want 1", report.AppPortCount)
	}
	if !report.AppPortDynamic {
		t.Error("AppPortDynamic = false, want true")
	}
	if !report.DevelopmentPortAbsent {
		t.Error("DevelopmentPortAbsent = false, want true")
	}
	if report.PostgresPortCount != 1 || !report.PostgresPortDynamic {
		t.Errorf("postgres port count=%d dynamic=%v, want 1/true", report.PostgresPortCount, report.PostgresPortDynamic)
	}
}

func TestValidateIsolationFromConfig_BrokenMerge_NotSatisfied(t *testing.T) {
	report, err := validateIsolationFromConfig(brokenMergeConfig)
	if err != nil {
		t.Fatalf("validateIsolationFromConfig: %v", err)
	}
	if report.Satisfied {
		t.Fatal("Satisfied = true for a config with the base development port still present -- the detector failed to catch a broken merge")
	}
	if report.AppPortCount != 2 {
		t.Errorf("AppPortCount = %d, want 2 (both the leaked base entry and the override entry)", report.AppPortCount)
	}
	if report.DevelopmentPortAbsent {
		t.Error("DevelopmentPortAbsent = true, want false -- published:\"8080\" is present in this fixture")
	}
	if len(report.Problems) == 0 {
		t.Error("expected at least one Problem to be recorded")
	}
}

func TestValidateIsolationFromConfig_MissingService_NotSatisfied(t *testing.T) {
	report, err := validateIsolationFromConfig("name: cnsdp-verify\nservices: {}\n")
	if err != nil {
		t.Fatalf("validateIsolationFromConfig: %v", err)
	}
	if report.Satisfied {
		t.Fatal("Satisfied = true with no services rendered at all")
	}
}

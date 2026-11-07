package workbench

import (
	"testing"
	"time"
)

func TestDeployCheckValidityAndTimeout(t *testing.T) {
	now := time.Now()
	free := uint64(1024)
	for _, tc := range []struct {
		status string
		age    time.Duration
		valid  bool
	}{
		{"passed", time.Hour, true}, {"passed", 24 * time.Hour, false}, {"passed", -time.Second, false},
		{"failed", time.Hour, false}, {"running", time.Minute, false},
	} {
		check := DeployCheck{Status: tc.status, At: now.Add(-tc.age), OK: true, FreeBytes: &free}
		if check.ValidAt(now) != tc.valid {
			t.Fatalf("unexpected validity for %+v", tc)
		}
	}
	check := DeployCheck{Status: "running", At: now.Add(-DeployCheckTimeout)}.DisplayAt(now)
	if check.Status != "failed" || check.OK {
		t.Fatal("expired in-flight check must fail closed")
	}
}

func TestDeployCheckRequiresMeasuredDiskCapacity(t *testing.T) {
	now := time.Now()
	check := DeployCheck{Status: "passed", At: now, OK: true}
	if check.ValidAt(now) || check.DisplayAt(now).Status != "expired" {
		t.Fatal("old check without disk measurement must be rechecked")
	}
	zero := uint64(0)
	check.FreeBytes = &zero
	if check.ValidAt(now) {
		t.Fatal("exhausted disk passed")
	}
}

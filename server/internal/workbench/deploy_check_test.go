package workbench

import (
	"testing"
	"time"
)

func TestDeployCheckValidityAndTimeout(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		status string
		age    time.Duration
		valid  bool
	}{
		{"passed", time.Hour, true}, {"passed", 24 * time.Hour, false}, {"passed", -time.Second, false},
		{"failed", time.Hour, false}, {"running", time.Minute, false},
	} {
		check := DeployCheck{Status: tc.status, At: now.Add(-tc.age), OK: true}
		if check.ValidAt(now) != tc.valid {
			t.Fatalf("unexpected validity for %+v", tc)
		}
	}
	check := DeployCheck{Status: "running", At: now.Add(-DeployCheckTimeout)}.DisplayAt(now)
	if check.Status != "failed" || check.OK {
		t.Fatal("expired in-flight check must fail closed")
	}
}

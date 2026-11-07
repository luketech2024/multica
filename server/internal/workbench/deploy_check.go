package workbench

import (
	"encoding/json"
	"time"
)

const DeployCheckValidity = 24 * time.Hour
const DeployCheckTimeout = 2 * time.Minute

type DeployCheck struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	At          time.Time `json:"at"`
	OK          bool      `json:"ok"`
	MS          int64     `json:"ms"`
	Detail      string    `json:"detail"`
	RequestedBy string    `json:"requested_by"`
}

func ParseDeployCheck(raw []byte) DeployCheck {
	var check DeployCheck
	_ = json.Unmarshal(raw, &check)
	return check
}

func (check DeployCheck) ValidAt(now time.Time) bool {
	return check.Status == "passed" && check.OK && !check.At.After(now) && check.At.After(now.Add(-DeployCheckValidity))
}

func (check DeployCheck) DisplayAt(now time.Time) DeployCheck {
	if check.Status == "passed" && !check.ValidAt(now) {
		check.Status, check.OK = "expired", false
	}
	if (check.Status == "queued" || check.Status == "running") && !check.At.After(now.Add(-DeployCheckTimeout)) {
		check.Status, check.OK, check.Detail = "failed", false, "runtime check timed out; verify the daemon connection and retry"
	}
	return check
}

package workbench

type ReleaseConfirmations struct {
	Changes        bool   `json:"changes"`
	Migration      bool   `json:"migration"`
	SecurityWaiver bool   `json:"security_waiver"`
	By             string `json:"by"`
	At             string `json:"at"`
	RoleRevision   int64  `json:"role_revision"`
}

type ReleaseGateBlocker struct {
	Code       string `json:"code"`
	ResourceID string `json:"resource_id,omitempty"`
}

type ReleaseGate struct {
	ID       string               `json:"id"`
	Passed   bool                 `json:"passed"`
	Required bool                 `json:"required"`
	Blockers []ReleaseGateBlocker `json:"blockers"`
}

func NewReleaseGate(id string) ReleaseGate {
	return ReleaseGate{ID: id, Passed: true, Required: true, Blockers: []ReleaseGateBlocker{}}
}

func (gate *ReleaseGate) Block(code, resourceID string) {
	gate.Passed = false
	gate.Blockers = append(gate.Blockers, ReleaseGateBlocker{Code: code, ResourceID: resourceID})
}

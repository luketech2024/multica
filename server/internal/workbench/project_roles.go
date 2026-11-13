package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ProjectRoleOrder = []string{"sched", "product", "dev", "test", "sec", "ops", "docs", "git"}

type ProjectRole struct {
	Enabled       bool    `json:"enabled"`
	AgentID       *string `json:"agent_id"`
	RuntimeID     *string `json:"runtime_id"`
	BindingSource string  `json:"binding_source,omitempty"`
}

type ProjectRoles struct {
	V     int                    `json:"v"`
	Roles map[string]ProjectRole `json:"roles"`
}

func DefaultProjectRoles() ProjectRoles {
	config := ProjectRoles{V: 1, Roles: make(map[string]ProjectRole)}
	for _, role := range ProjectRoleOrder {
		config.Roles[role] = ProjectRole{Enabled: true}
	}
	return config
}

func ParseProjectRoles(raw []byte) (ProjectRoles, error) {
	var config ProjectRoles
	if err := json.Unmarshal(raw, &config); err != nil || config.V != 1 || config.Roles == nil {
		return config, errors.New("project role configuration must use version 1")
	}
	for _, role := range []string{"docs", "git"} {
		if _, exists := config.Roles[role]; !exists {
			config.Roles[role] = ProjectRole{Enabled: false}
		}
	}
	if len(config.Roles) != len(ProjectRoleOrder) {
		return config, errors.New("unknown project role")
	}
	for _, role := range ProjectRoleOrder {
		entry, exists := config.Roles[role]
		if !exists {
			return config, fmt.Errorf("missing project role %s", role)
		}
		if entry.BindingSource != "" && entry.BindingSource != "explicit" && entry.BindingSource != "onespec_default" {
			return config, fmt.Errorf("invalid binding source for role %s", role)
		}
		if (role == "sched" || role == "product") && !entry.Enabled {
			return config, errors.New("scheduling and product roles must remain enabled")
		}
	}
	return config, nil
}

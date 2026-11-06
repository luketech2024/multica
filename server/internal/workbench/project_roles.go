package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ProjectRoleOrder = []string{"sched", "product", "dev", "test", "sec", "ops"}

type ProjectRole struct {
	Enabled   bool    `json:"enabled"`
	AgentID   *string `json:"agent_id"`
	RuntimeID *string `json:"runtime_id"`
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
	if err := json.Unmarshal(raw, &config); err != nil || config.V != 1 {
		return config, errors.New("project role configuration must use version 1")
	}
	if len(config.Roles) != len(ProjectRoleOrder) {
		return config, errors.New("configure all six project roles")
	}
	for _, role := range ProjectRoleOrder {
		entry, exists := config.Roles[role]
		if !exists {
			return config, fmt.Errorf("missing project role %s", role)
		}
		if (role == "sched" || role == "product") && !entry.Enabled {
			return config, errors.New("scheduling and product roles must remain enabled")
		}
	}
	return config, nil
}

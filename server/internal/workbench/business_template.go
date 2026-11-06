package workbench

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type TemplateProhibition struct {
	Rule  string `json:"rule"`
	Check string `json:"check"`
}

type TemplateReference struct {
	Type    string `json:"type"`
	Ref     string `json:"ref"`
	Version string `json:"version"`
}

type BusinessTemplate struct {
	V                  int                   `json:"v"`
	Name               string                `json:"name"`
	Background         []string              `json:"background"`
	Audience           []string              `json:"audience"`
	ScopeOut           []string              `json:"scope_out"`
	AcceptanceSkeleton []string              `json:"ac_skeleton"`
	Forbidden          []TemplateProhibition `json:"forbidden"`
	DefinitionOfDone   []string              `json:"dod"`
	Resources          []TemplateReference   `json:"resources"`
	Terms              map[string]string     `json:"terms"`
}

type BusinessTemplateAsset struct {
	ID            string           `json:"id"`
	Revision      int64            `json:"revision"`
	Content       BusinessTemplate `json:"content"`
	DisabledRoles []string         `json:"disabled_roles"`
}

type ProjectTemplate struct {
	SourceID       string           `json:"source_id"`
	SourceRevision int64            `json:"source_revision"`
	Revision       int64            `json:"revision"`
	Content        BusinessTemplate `json:"content"`
}

//go:embed business_templates.json
var businessTemplateCatalog []byte

func BuiltinBusinessTemplates() ([]BusinessTemplateAsset, error) {
	var assets []BusinessTemplateAsset
	if err := json.Unmarshal(businessTemplateCatalog, &assets); err != nil {
		return nil, err
	}
	for _, asset := range assets {
		if err := ValidateBusinessTemplate(asset.Content); err != nil {
			return nil, fmt.Errorf("template %s: %w", asset.ID, err)
		}
	}
	return assets, nil
}

func ValidateBusinessTemplate(template BusinessTemplate) error {
	if template.Background == nil || template.Audience == nil || template.ScopeOut == nil || template.AcceptanceSkeleton == nil || template.Forbidden == nil || template.DefinitionOfDone == nil || template.Resources == nil || template.Terms == nil {
		return errors.New("template requires all eight fields; use empty lists or an empty terms object when unused")
	}
	if template.V != 1 || strings.TrimSpace(template.Name) == "" || utf8.RuneCountInString(template.Name) > 80 {
		return errors.New("template requires version 1 and a name of 1 to 80 characters")
	}
	if len(template.Background) != 0 && len(template.Background) != 4 {
		return errors.New("template background must have four sections")
	}
	for _, field := range []struct {
		name   string
		values []string
		limit  int
	}{
		{"background", template.Background, 2000},
		{"audience", template.Audience, 500},
		{"scope_out", template.ScopeOut, 500},
		{"ac_skeleton", template.AcceptanceSkeleton, 120},
		{"dod", template.DefinitionOfDone, 500},
	} {
		if len(field.values) > 100 {
			return fmt.Errorf("template %s has too many entries", field.name)
		}
		for _, value := range field.values {
			if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > field.limit {
				return fmt.Errorf("template %s entries must contain 1 to %d characters", field.name, field.limit)
			}
		}
	}
	if len(template.Forbidden) > 100 || len(template.Resources) > 100 || len(template.Terms) > 100 {
		return errors.New("template contains too many rules, references or terms")
	}
	for _, rule := range template.Forbidden {
		if strings.TrimSpace(rule.Rule) == "" || strings.TrimSpace(rule.Check) == "" {
			return errors.New("every prohibition requires a rule and a check method")
		}
		if utf8.RuneCountInString(rule.Rule) > 500 || utf8.RuneCountInString(rule.Check) > 500 {
			return errors.New("prohibition rules and check methods must not exceed 500 characters")
		}
	}
	for _, reference := range template.Resources {
		if strings.TrimSpace(reference.Ref) == "" || strings.TrimSpace(reference.Version) == "" {
			return errors.New("template references require a path and version")
		}
	}
	for term, meaning := range template.Terms {
		if strings.TrimSpace(term) == "" || strings.TrimSpace(meaning) == "" {
			return errors.New("template terms require a name and meaning")
		}
	}
	return nil
}

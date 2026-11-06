package workbench

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBusinessTemplateCatalogAndValidation(t *testing.T) {
	assets, err := BuiltinBusinessTemplates()
	if err != nil || len(assets) != 4 {
		t.Fatalf("invalid built-in catalog: %v", err)
	}
	content := assets[0].Content
	content.Forbidden[0].Check = " "
	if ValidateBusinessTemplate(content) == nil {
		t.Fatal("a prohibition without a check method was accepted")
	}
	fresh, _ := BuiltinBusinessTemplates()
	if fresh[0].Content.Forbidden[0].Check == " " {
		t.Fatal("editing a project copy changed the built-in template")
	}
	missing := fresh[0].Content
	missing.Terms = nil
	if ValidateBusinessTemplate(missing) == nil {
		t.Fatal("missing template fields were accepted")
	}
}

func TestBusinessTemplatePlaceholdersCannotPassRequirementsConfirmation(t *testing.T) {
	for _, text := range []string{"{功能点} 符合设计要求", "{} 接口联调通过"} {
		input, _ := json.Marshal(Requirements{V: 1, Items: []Item{{Text: text}}})
		stored, err := EditRequirements(nil, input, false, "member", "member", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		criteria, _ := ParseRequirements(stored)
		criteria.Items[0].Checked = &Check{}
		checked, _ := json.Marshal(criteria)
		if _, err := EditRequirements(stored, checked, true, "member", "member", time.Now()); err == nil {
			t.Fatal("unreplaced template placeholder passed confirmation")
		}
	}
}

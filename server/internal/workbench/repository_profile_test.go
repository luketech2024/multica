package workbench

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRepositoryProfileDerivesCommandsAndKeepsManualOverridesSeparate(t *testing.T) {
	profile, err := BuildRepositoryProfile(RepositoryProfileInput{Template: "web-frontend", Tags: []string{"java21", "api", "integration-test", "offline"}, Delivery: []string{"docker-image"}, Overrides: map[string]string{"test": "./scripts/verify.sh"}}, RepositoryProfile{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Stack.Template != "java-backend" || profile.Stack.Derived.Test != "mvn -q -B verify" || !strings.Contains(profile.Stack.Derived.Install, "$MULTICA_MAVEN_SETTINGS") {
		t.Fatalf("incorrect derived commands: %+v", profile.Stack)
	}
	if profile.Stack.Overrides["test"].By != "manual" || profile.Stack.Overrides["test"].Value != "./scripts/verify.sh" || profile.Stack.Detected != nil || profile.Stack.Baseline != nil {
		t.Fatal("manual provenance or unverified scan state lost")
	}
	for _, template := range []string{"java-backend", "java8-legacy", "python-service", "ai-app", "weapp", "web-frontend", "custom"} {
		if _, err := BuildRepositoryProfile(RepositoryProfileInput{Template: template, Delivery: []string{"none"}}, RepositoryProfile{}); err != nil {
			t.Fatal(err)
		}
	}
	weapp, err := BuildRepositoryProfile(RepositoryProfileInput{Template: "weapp", Tags: []string{"ts", "node20", "weapp"}, Delivery: []string{"weapp-ci"}}, RepositoryProfile{})
	if err != nil || weapp.Stack.Template != "weapp" || weapp.Stack.Runtime != "local" {
		t.Fatalf("weapp runtime lost: %+v, %v", weapp, err)
	}
}

func TestRepositoryProfileRejectsConflictsAndUnknownValues(t *testing.T) {
	for _, input := range []RepositoryProfileInput{
		{Template: "unknown", Delivery: []string{"none"}},
		{Template: "custom", Tags: []string{"java21", "java8"}, Delivery: []string{"none"}},
		{Template: "custom", Tags: []string{"java21", "node20"}, Delivery: []string{"none"}},
		{Template: "custom", Tags: []string{"unknown"}, Delivery: []string{"none"}},
		{Template: "custom", Delivery: []string{"none", "static-site"}},
		{Template: "custom", Delivery: []string{}},
		{Template: "custom", Delivery: []string{"unknown"}},
		{Template: "custom", Delivery: []string{"none"}, Overrides: map[string]string{"publish": "go"}},
	} {
		if _, err := BuildRepositoryProfile(input, RepositoryProfile{}); err == nil {
			t.Fatalf("accepted invalid input %+v", input)
		}
	}
}

func TestRepositoryProfileSurvivesOrdinaryEditsAndInvalidatesChangedSources(t *testing.T) {
	profile, err := BuildRepositoryProfile(RepositoryProfileInput{Template: "python-service", Delivery: []string{"service-process"}}, RepositoryProfile{})
	if err != nil {
		t.Fatal(err)
	}
	profile.Stack.Detected = json.RawMessage(`{"lang":"python"}`)
	profile.Stack.Baseline = json.RawMessage(`{"scanned_at":"2026-11-07"}`)
	raw, err := WriteRepositoryProfile([]byte(`{"url":"https://example.test/api.git","ref":"main"}`), profile)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := PreserveRepositoryProfile(raw, []byte(`{"url":"https://example.test/api.git","ref":"main","stack":{"v":999}}`))
	if err != nil {
		t.Fatal(err)
	}
	read, err := ReadRepositoryProfile(kept)
	if err != nil || read.Revision != 1 || read.Stack.V != 3 || read.Stack.Detected == nil {
		t.Fatal("generic edit replaced the authoritative profile")
	}
	changed, err := PreserveRepositoryProfile(raw, []byte(`{"url":"https://example.test/other.git","ref":"main"}`))
	if err != nil {
		t.Fatal(err)
	}
	read, err = ReadRepositoryProfile(changed)
	if err != nil || read.Revision != 2 || string(read.Stack.Detected) != "null" || string(read.Stack.Baseline) != "null" {
		t.Fatalf("changed source kept old verification: %+v, %v", read, err)
	}
}

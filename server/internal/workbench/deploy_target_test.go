package workbench

import "testing"

func TestDeployTargetsCanBeSavedBeforeEnvironmentIsAvailable(t *testing.T) {
	target := DeployTargetInput{Label: "Staging node", Env: "staging", Kind: "docker-host", Channel: "runtime"}
	if err := target.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(target.MissingFields()) != 5 {
		t.Fatalf("missing fields: %v", target.MissingFields())
	}
	target.Location = map[string]string{"host": "node.internal", "image": "registry.internal/app", "dir": "/srv/app", "health_url": "https://app.internal/health"}
	runtime := "runtime-id"
	target.RuntimeID = &runtime
	if err := target.Validate(); err != nil || len(target.MissingFields()) != 0 {
		t.Fatalf("complete configuration: %v, %v", err, target.MissingFields())
	}
}

func TestDeployTargetRejectsInvalidChannelsAndCredentialURLs(t *testing.T) {
	for _, input := range []DeployTargetInput{
		{Label: "Node", Env: "prod", Kind: "docker-host", Channel: "platform"},
		{Label: "Website", Env: "prod", Kind: "static-site", Channel: "ssh"},
		{Label: "Node", Env: "unknown", Kind: "docker-host", Channel: "runtime"},
		{Label: "Node", Env: "prod", Kind: "docker-host", Channel: "runtime", Location: map[string]string{"health_url": "https://secret:password@example.com/health"}},
		{Label: "Node", Env: "prod", Kind: "docker-host", Channel: "runtime", Location: map[string]string{"secret": "do-not-store-here"}},
	} {
		if err := input.Validate(); err == nil {
			t.Fatalf("accepted invalid target %+v", input)
		}
	}
}

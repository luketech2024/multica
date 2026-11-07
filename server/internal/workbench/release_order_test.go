package workbench

import "testing"

func TestReleaseRequestNormalization(t *testing.T) {
	input := ReleaseOrderInput{Env: "staging", Tasks: []ReleaseTaskInput{{ID: "b", ExpectedRevision: 2}, {ID: "a", ExpectedRevision: 1}}}
	if err := input.Normalize(); err != nil {
		t.Fatal(err)
	}
	digest := input.Digest()
	input.Tasks[0], input.Tasks[1] = input.Tasks[1], input.Tasks[0]
	if err := input.Normalize(); err != nil || input.Digest() != digest {
		t.Fatal("task selection order changed request identity")
	}
	for _, tag := range []string{"v1..2", "v1.lock", "v1.", "--bad"} {
		input.Tags = map[string]string{"repo": tag}
		if input.Normalize() == nil {
			t.Fatalf("accepted invalid tag %q", tag)
		}
	}
	input.Tags = map[string]string{"repo": "2026.11.07-1"}
	if err := input.Normalize(); err != nil {
		t.Fatal(err)
	}
	input.Env = "preview"
	if input.Normalize() == nil {
		t.Fatal("preview became a release order")
	}
	input.Env = "prod"
	input.Tasks[1] = input.Tasks[0]
	if input.Normalize() == nil {
		t.Fatal("duplicate task accepted")
	}
}

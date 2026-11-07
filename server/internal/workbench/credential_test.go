package workbench

import (
	"bytes"
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	"testing"
)

func TestCredentialEnvelopeEncryptsSecretAndIdentity(t *testing.T) {
	master, err := secretbox.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed, wrapped, err := SealCredential(master, "credential-1", "workspace-1", "test-only-secret")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("test-only-secret")) {
		t.Fatal("plaintext stored")
	}
	key, err := master.Open(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	box, err := secretbox.New(key)
	if err != nil {
		t.Fatal(err)
	}
	clear(key)
	payload, err := box.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	var value struct{ ID, WorkspaceID, Secret string }
	if err = json.Unmarshal(payload, &value); err != nil {
		t.Fatal(err)
	}
	if value.ID != "credential-1" || value.WorkspaceID != "workspace-1" || value.Secret != "test-only-secret" {
		t.Fatal("credential identity or secret changed")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err = box.Open(sealed); err == nil {
		t.Fatal("tampered credential decrypted")
	}
}

func TestCredentialUsageSeparatesDeploymentFromRepositoryWrites(t *testing.T) {
	input := CredentialInput{Label: "Test token", Kind: "token", Secret: "test-only-secret", Usage: []string{"deploy", "read-repo"}}
	if input.Validate() == nil {
		t.Fatal("mixed deployment and repository credential accepted")
	}
	input.Usage = []string{"read-repo", "push-branch", "merge"}
	if err := input.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCredentialRejectsIdentitySwapAndTampering(t *testing.T) {
	box, _ := secretbox.New(bytes.Repeat([]byte{5}, 32))
	sealed, wrapped, _ := SealCredential(box, "credential", "workspace", "test-only")
	secret, err := OpenCredential(box, "credential", "workspace", sealed, wrapped)
	if err != nil || string(secret) != "test-only" {
		t.Fatal("could not open valid envelope")
	}
	clear(secret)
	if _, err := OpenCredential(box, "other", "workspace", sealed, wrapped); err == nil {
		t.Fatal("credential swap accepted")
	}
	if _, err := OpenCredential(box, "credential", "other", sealed, wrapped); err == nil {
		t.Fatal("workspace swap accepted")
	}
	sealed[0] ^= 1
	if _, err := OpenCredential(box, "credential", "workspace", sealed, wrapped); err == nil {
		t.Fatal("tampered secret accepted")
	}
}

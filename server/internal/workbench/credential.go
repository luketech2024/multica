package workbench

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/multica-ai/multica/server/internal/util/secretbox"
)

type CredentialInput struct {
	Label            string   `json:"label"`
	Kind             string   `json:"kind"`
	ProjectID        *string  `json:"project_id"`
	Usage            []string `json:"usage"`
	Secret           string   `json:"secret"`
	ExpiresAt        *string  `json:"expires_at"`
	ExpectedRevision int64    `json:"expected_revision"`
}

func (input *CredentialInput) Validate() error {
	input.Label = strings.TrimSpace(input.Label)
	if input.Label == "" || len([]rune(input.Label)) > 120 {
		return errors.New("credential label must contain 1 to 120 characters")
	}
	if !slices.Contains([]string{"ssh-key", "ssh-password", "token", "cloud-ak", "weapp-ci-key", "registry-token"}, input.Kind) {
		return errors.New("unsupported credential kind")
	}
	if len(input.Usage) == 0 || len(input.Usage) > 4 {
		return errors.New("select at least one credential usage")
	}
	seen := map[string]bool{}
	for _, usage := range input.Usage {
		if !slices.Contains([]string{"read-repo", "push-branch", "merge", "deploy"}, usage) || seen[usage] {
			return errors.New("invalid or duplicate credential usage")
		}
		seen[usage] = true
	}
	if seen["deploy"] && len(input.Usage) != 1 {
		return errors.New("deployment credentials cannot be reused for repository access")
	}
	if input.Kind == "ssh-password" && !seen["deploy"] {
		return errors.New("SSH passwords are for deployment use only")
	}
	if strings.TrimSpace(input.Secret) == "" || len(input.Secret) > 65536 {
		return errors.New("credential secret must contain 1 to 65536 bytes")
	}
	return nil
}

// Bind the encrypted payload to its identity so swapping rows cannot exchange secrets.
func SealCredential(master *secretbox.Box, id, workspaceID, secret string) (ciphertext, wrappedKey []byte, err error) {
	key := make([]byte, secretbox.KeySize)
	if _, err = rand.Read(key); err != nil {
		return nil, nil, err
	}
	defer clear(key)
	dataBox, err := secretbox.New(key)
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(struct{ ID, WorkspaceID, Secret string }{id, workspaceID, secret})
	if err != nil {
		return nil, nil, err
	}
	defer clear(payload)
	ciphertext, err = dataBox.Seal(payload)
	if err != nil {
		return nil, nil, err
	}
	wrappedKey, err = master.Seal(key)
	return ciphertext, wrappedKey, err
}

// OpenCredential verifies the envelope identity before exposing the secret to its consumer.
func OpenCredential(master *secretbox.Box, id, workspaceID string, ciphertext, wrappedKey []byte) ([]byte, error) {
	if master == nil {
		return nil, errors.New("credential encryption is not configured")
	}
	key, err := master.Open(wrappedKey)
	if err != nil {
		return nil, errors.New("credential key could not be decrypted")
	}
	defer clear(key)
	box, err := secretbox.New(key)
	if err != nil {
		return nil, err
	}
	payload, err := box.Open(ciphertext)
	if err != nil {
		return nil, errors.New("credential could not be decrypted")
	}
	defer clear(payload)
	var value struct{ ID, WorkspaceID, Secret string }
	if json.Unmarshal(payload, &value) != nil || value.ID != id || value.WorkspaceID != workspaceID {
		return nil, errors.New("credential identity does not match")
	}
	return []byte(value.Secret), nil
}

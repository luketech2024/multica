package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

func init() {
	command := &cobra.Command{Use: "onespec", Short: "Inspect and update the versioned OneSpec workflow"}
	for _, action := range []string{"get", "start", "kind", "artifact", "decision", "advance", "reopen", "git", "run", "release-draft", "refresh-configuration"} {
		child := &cobra.Command{Use: action + " <issue-id>", Short: action + " a OneSpec workflow", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := cli.APIContext(context.Background())
			defer cancel()
			ref, err := resolveIssueRef(ctx, client, args[0])
			if err != nil {
				return err
			}
			endpoint := "/api/issues/" + ref.ID + "/onespec"
			var result map[string]any
			if action == "get" {
				err = client.GetJSON(ctx, endpoint, &result)
			} else {
				file, _ := cmd.Flags().GetString("file")
				raw, e := os.ReadFile(file)
				if e != nil {
					return e
				}
				var input map[string]any
				if json.Unmarshal(raw, &input) != nil || input == nil {
					return fmt.Errorf("file must contain a JSON object")
				}
				if _, ok := input["expected_revision"]; !ok {
					return fmt.Errorf("expected_revision is required")
				}
				if action == "artifact" {
					directory, _ := cmd.Flags().GetString("repository-dir")
					if directory != "" {
						if err := attachRequirementDocuments(ctx, input, directory); err != nil {
							return err
						}
					}
				}
				err = client.PostJSON(ctx, endpoint+"/"+action, input, &result)
			}
			if err != nil {
				return err
			}
			return cli.PrintJSON(os.Stdout, result)
		}}
		if action != "get" {
			child.Flags().String("file", "", "UTF-8 JSON request with expected_revision")
			_ = child.MarkFlagRequired("file")
			if action == "artifact" {
				child.Flags().String("repository-dir", "", "Read PRD and optional prototype from the referenced Git commits in this checkout")
			}
		}
		command.AddCommand(child)
	}
	issueCmd.AddCommand(command)
}

func attachRequirementDocuments(ctx context.Context, input map[string]any, directory string) error {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	artifact, ok := input["artifact"].(map[string]any)
	if !ok || artifact["stage"] != "requirements" {
		return fmt.Errorf("repository-dir requires a requirements artifact")
	}
	document, ok := artifact["document"].(map[string]any)
	if !ok {
		return fmt.Errorf("provide artifact.document.ref and optional document.prototype.ref")
	}
	documents := []map[string]any{document}
	if prototype, ok := document["prototype"].(map[string]any); ok {
		document["prototype_status"] = "ready"
		documents = append(documents, prototype)
	}
	refs := []any{}
	for _, file := range documents {
		ref, ok := file["ref"].(map[string]any)
		if !ok {
			return fmt.Errorf("document.ref is required")
		}
		commit, _ := ref["commit"].(string)
		path, _ := ref["path"].(string)
		if commit == "" || path == "" {
			return fmt.Errorf("document commit and path are required")
		}
		raw, err := exec.CommandContext(ctx, "git", "-C", directory, "show", commit+":"+path).Output()
		if err != nil {
			return fmt.Errorf("read committed document %s: %w", path, err)
		}
		digest := sha256.Sum256(raw)
		ref["digest"] = hex.EncodeToString(digest[:])
		file["content"], file["checkout_path"] = string(raw), directory
		refs = append(refs, ref)
	}
	artifact["refs"] = refs
	return nil
}

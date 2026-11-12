package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
		}
		command.AddCommand(child)
	}
	issueCmd.AddCommand(command)
}

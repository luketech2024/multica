package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/workbench"
	"github.com/spf13/cobra"
)

func init() {
	workflow := &cobra.Command{Use: "workflow", Short: "Save structured requirements, execution plans and delivery evidence"}
	for _, action := range []string{"requirements", "plan", "delivery"} {
		command := &cobra.Command{
			Use:   action + " <issue-id> --file <json-file>",
			Short: "Save " + action + " through the versioned issue workflow API",
			Long: `Read the issue first. The UTF-8 JSON file must contain expected_revision.
requirements: {expected_revision, acceptance_criteria: {v:1, phase:"draft"|"clarifying"|"ready", covered_discussion_revision:<revision read>, items:[{text:"..."}], bounds:[]}}.
Before saving, run multica issue comment list <issue-id> --full --output json and read its complete output. Summary, roots-only and folded reads do not record full discussion coverage. Use acceptance_criteria.discussion_revision from issue get (omitted means 0). A draft does not request confirmation; phase ready explicitly submits it.
Preserve existing item IDs when updating. Only a human may confirm requirements.
plan: {expected_revision:1, version:1, idempotency_key:"stable-key", tasks:[{key:"analyze",title:"Analyze deployment",description:"Describe startup requirements",role:"ops",repo_id:"project-resource-id",covers:["A-01"],depends_on:[],serial_group:"deployment",expected:["Deployment report"]}]}.
These are top-level fields, without a plan wrapper. expected, covers and depends_on are arrays of strings. Include only roles needed for the confirmed scope. Every role in an automated plan needs a bound, accessible agent. Report missing bindings before preparing the full plan and wait for the project team to be updated.
delivery: {expected_revision, evidence:{issue_id,issue_revision,commit,passed,summary,reports:[{name,url}],changes:{files,additions,deletions},preview_url}}.
Delivery is submitted on the parent issue and identifies the completed child. Use real report URLs and full commit hashes. A successful report finalizes that child's delivery; human acceptance belongs to the parent.`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				path, _ := cmd.Flags().GetString("file")
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				var input map[string]any
				if json.Unmarshal(raw, &input) != nil || input == nil {
					return fmt.Errorf("file must contain a JSON object")
				}
				if revision, ok := input["expected_revision"].(float64); !ok || revision < 1 {
					return fmt.Errorf("expected_revision is required")
				}
				if action == "plan" {
					if _, err := workbench.ParseExecutionPlanRequest(raw); err != nil {
						return err
					}
				}
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
				endpoint := "/api/issues/" + ref.ID
				var result map[string]any
				switch action {
				case "requirements":
					err = client.PutJSON(ctx, endpoint, input, &result)
				case "plan":
					err = client.PostJSON(ctx, endpoint+"/execution-plan", input, &result)
				case "delivery":
					err = client.PostJSON(ctx, endpoint+"/delivery-evidence", input, &result)
				}
				if err != nil {
					return err
				}
				return cli.PrintJSON(os.Stdout, result)
			},
		}
		command.Flags().String("file", "", "UTF-8 JSON request file")
		_ = command.MarkFlagRequired("file")
		workflow.AddCommand(command)
	}
	issueCmd.AddCommand(workflow)
}

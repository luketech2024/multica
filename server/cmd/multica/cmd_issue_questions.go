package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

func init() {
	command := &cobra.Command{
		Use:   "questions <issue-id>",
		Short: "Read questions or submit choices for the user with --file",
		Long: `Without --file, read questions and answers. With --file, submit
{"key":"stable-key","questions":[{"id":"scope","header":"Scope","question":"What should we deliver?","isOther":true,"options":[{"label":"Basic setup","description":"Verify login","recommended":true}]}]}.
Use a new key for revised questions. The user answers in the task conversation.
After submitting, end this turn and wait for the answer. Never confirm requirements for the user.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			endpoint := "/api/issues/" + ref.ID + "/input-requests"
			file, _ := cmd.Flags().GetString("file")
			var result any
			if file == "" {
				err = client.GetJSON(ctx, endpoint, &result)
			} else {
				raw, readErr := os.ReadFile(file)
				if readErr != nil {
					return readErr
				}
				var request protocol.UserInputRequest
				if json.Unmarshal(raw, &request) != nil {
					return fmt.Errorf("invalid questions JSON")
				}
				request.Origin = "platform"
				if err := request.Validate(); err != nil {
					return err
				}
				err = client.PostJSON(ctx, endpoint, request, &result)
			}
			if err != nil {
				return err
			}
			return cli.PrintJSON(os.Stdout, result)
		},
	}
	command.Flags().String("file", "", "UTF-8 JSON questions file")
	issueCmd.AddCommand(command)
}

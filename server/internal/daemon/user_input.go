package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

type taskUserInput struct {
	ID       string                     `json:"id"`
	Status   string                     `json:"status"`
	Response protocol.UserInputResponse `json:"response"`
}

func (d *Daemon) requestUserInput(ctx context.Context, taskID string, request protocol.UserInputRequest, deliver func(protocol.UserInputResponse) error) error {
	var row taskUserInput
	delivered := false
	path := fmt.Sprintf("/api/daemon/tasks/%s/input-requests", taskID)
	if err := d.client.postJSONWithRetry(ctx, path, request, &row, []time.Duration{0, 200 * time.Millisecond, time.Second}); err != nil {
		return err
	}
	path += "/" + row.ID
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		status := "cancelled"
		if delivered {
			status = "resolved"
		}
		if err := d.client.postJSON(cleanup, path+"/resolve", map[string]string{"status": status}, nil); err != nil {
			d.logger.Warn("failed to resolve user input", "task_id", taskID, "request_id", row.ID)
		}
	}()
	for {
		switch row.Status {
		case "answered":
			err := deliver(row.Response)
			delivered = err == nil
			return err
		case "cancelled", "resolved":
			return fmt.Errorf("question is no longer active")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		if err := d.client.getJSON(ctx, path, &row); err != nil {
			return err
		}
	}
}

package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-shellwords"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (d *Daemon) handleDeployTargetCheck(ctx context.Context, runtime Runtime, check protocol.DaemonDeployCheck) {
	checkContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	result := probeDeploymentTarget(checkContext, check)
	cancel()
	reportContext, cancelReport := context.WithTimeout(ctx, 20*time.Second)
	defer cancelReport()
	path := fmt.Sprintf("/api/daemon/runtimes/%s/deploy-targets/%s/checks/%s", runtime.ID, check.TargetID, check.CheckID)
	if err := d.client.postJSONWithRetry(reportContext, path, result, nil, defaultTerminalRetrySchedule); err != nil {
		d.logger.Warn("deployment check report failed", "target_id", check.TargetID, "check_id", check.CheckID, "error", err)
	}
}

func probeDeploymentTarget(ctx context.Context, check protocol.DaemonDeployCheck) protocol.DaemonDeployCheckResult {
	started := time.Now()
	result := protocol.DaemonDeployCheckResult{}
	if filepath.IsAbs(check.Directory) {
		file, err := os.CreateTemp(check.Directory, ".multica-deploy-check-*")
		if err == nil {
			_, writeErr := file.Write([]byte("deployment check"))
			closeErr := file.Close()
			removeErr := os.Remove(file.Name())
			result.DirectoryReady = writeErr == nil && closeErr == nil && removeErr == nil
		}
	}
	switch check.Kind {
	case "docker-host":
		result.ToolReady = exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").Run() == nil
	case "service-process":
		parser := shellwords.NewParser()
		parser.ParseEnv, parser.ParseBacktick = false, false
		words, err := parser.Parse(check.StartCommand)
		if err == nil && len(words) > 0 {
			command := words[0]
			if !filepath.IsAbs(command) && strings.ContainsAny(command, `/\`) {
				command = filepath.Join(check.Directory, command)
			}
			_, err = exec.LookPath(command)
			result.ToolReady = err == nil
		}
	}
	result.MS = time.Since(started).Milliseconds()
	return result
}

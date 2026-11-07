package daemon

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestDeployCheckUsesDockerDaemonProbe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires Unix")
	}
	bin := t.TempDir()
	tool := filepath.Join(bin, "docker")
	writeTestExecutable(t, tool, []byte("#!/bin/sh\n[ \"$1\" = info ] && [ \"$2\" = --format ]\n"))
	t.Setenv("PATH", bin)
	check := protocol.DaemonDeployCheck{Kind: "docker-host", Directory: t.TempDir()}
	if result := probeDeploymentTarget(context.Background(), check); !result.DirectoryReady || !result.ToolReady {
		t.Fatalf("fixture docker probe failed: %+v", result)
	}
	writeTestExecutable(t, tool, []byte("#!/bin/sh\nexit 1\n"))
	if result := probeDeploymentTarget(context.Background(), check); result.ToolReady {
		t.Fatal("failed docker daemon probe passed")
	}
}

func TestDeployCheckProbesDirectoryWithoutStartingService(t *testing.T) {
	directory := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	check := protocol.DaemonDeployCheck{Kind: "service-process", Directory: directory, StartCommand: strconv.Quote(executable) + " --never-execute"}
	result := probeDeploymentTarget(context.Background(), check)
	if !result.DirectoryReady || !result.ToolReady || result.FreeBytes == nil || *result.FreeBytes == 0 {
		t.Fatalf("probe failed: %+v", result)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatal("probe left files behind")
	}
	check.Directory = filepath.Join(directory, "missing")
	check.StartCommand = filepath.Join(directory, "missing-program")
	result = probeDeploymentTarget(context.Background(), check)
	if result.DirectoryReady || result.ToolReady || result.FreeBytes != nil {
		t.Fatal("missing directory/tool passed")
	}
}

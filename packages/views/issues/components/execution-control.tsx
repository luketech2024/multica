"use client";

import { useState } from "react";
import type { Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import {
  useControlIssueExecution,
  type ExecutionControlAction,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

export function ExecutionControl({ issue, pauseLabel }: { issue: Issue; pauseLabel?: string }) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const mutation = useControlIssueExecution(wsId, issue.id);
  const [error, setError] = useState<string>();
  const pause = issue.workbench_pause;
  if (
    !issue.revision ||
    !["owner", "admin", "member"].includes(member.role ?? "") ||
    (!issue.acceptance_criteria && !issue.workbench_handoff) ||
    issue.workbench_handoff?.superseded ||
    issue.acceptance_criteria?.change ||
    (issue.acceptance_criteria?.resume_runs?.length ?? 0) > 0 ||
    ["done", "cancelled"].includes(issue.status) ||
    (!pause &&
      !["backlog", "todo", "in_progress", "in_review", "blocked"].includes(
        issue.status,
      ))
  )
    return null;

  const act = async (action: ExecutionControlAction) => {
    setError(undefined);
    try {
      await mutation.mutateAsync({ action, revision: issue.revision! });
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.execution_control.failed),
      );
    }
  };
  return (
    <fieldset disabled={mutation.isPending} className="space-y-2 text-caption">
      {pause && (
        <p role="status">{t(($) => $.execution_control[pause.phase])}</p>
      )}
      {pause?.resume_error && (
        <p role="alert" className="text-destructive">
          {pause.resume_error}
        </p>
      )}
      {!pause && issue.status !== "paused" && (
        <Button
          size="sm"
          variant="outline"
          aria-busy={mutation.isPending}
          onClick={() => void act("pause")}
        >
          {pauseLabel ?? t(($) => $.execution_control.pause)}
        </Button>
      )}
      {(pause?.phase === "stopping" || pause?.phase === "unknown") && (
        <Button
          size="sm"
          variant="outline"
          onClick={() => void act("reconcile")}
        >
          {t(($) => $.execution_control.reconcile)}
        </Button>
      )}
      {(pause?.phase === "paused" || pause?.phase === "resuming") && (
        <Button
          size="sm"
          variant="outline"
          aria-busy={mutation.isPending}
          onClick={() => void act("resume")}
        >
          {t(($) => $.execution_control.resume)}
        </Button>
      )}
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
    </fieldset>
  );
}

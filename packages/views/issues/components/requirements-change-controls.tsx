"use client";

import { useState } from "react";
import type { Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { useChangeIssueRequirements } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";

export function RequirementsChangeControls({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const mutation = useChangeIssueRequirements(wsId, issue.id);
  const [requesting, setRequesting] = useState(false);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string>();
  const criteria = issue.acceptance_criteria;
  const change = criteria?.change;
  const needsResume = (criteria?.resume_runs?.length ?? 0) > 0;
  const canEdit = ["owner", "admin", "member"].includes(member.role ?? "");
  if (!canEdit || !criteria || issue.revision === undefined) return null;
  if (!change && (issue.status !== "in_progress" || !criteria.gate.spec))
    return null;
  const act = async (
    action: "start" | "reconcile" | "edit" | "cancel" | "resume",
  ) => {
    setError(undefined);
    try {
      await mutation.mutateAsync({ action, revision: issue.revision!, reason });
      setRequesting(false);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.requirements.save_failed),
      );
    }
  };
  return (
    <fieldset disabled={mutation.isPending} className="space-y-2 text-caption">
      {change && (
        <>
          <p className="font-medium" role="status">
            {t(($) => $.requirements_change[change.phase])}
          </p>
          <p className="text-muted-foreground break-words">{change.reason}</p>
          {(change.phase === "stopping" || change.phase === "unknown") && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => void act("reconcile")}
            >
              {t(($) => $.requirements_change.reconcile)}
            </Button>
          )}
          {change.phase === "paused" && (
            <Button size="sm" onClick={() => void act("edit")}>
              {t(($) => $.requirements_change.edit)}
            </Button>
          )}
          {(change.phase === "paused" || change.phase === "editing") && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => void act("cancel")}
            >
              {t(($) => $.requirements_change.cancel)}
            </Button>
          )}
        </>
      )}
      {needsResume && (
        <Button size="sm" onClick={() => void act("resume")}>
          {t(($) => $.requirements_change.resume)}
        </Button>
      )}
      {criteria.resume_error && (
        <p role="alert" className="text-destructive">
          {criteria.resume_error}
        </p>
      )}
      {!change && !requesting && !needsResume && (
        <Button size="sm" variant="outline" onClick={() => setRequesting(true)}>
          {t(($) => $.requirements_change.start)}
        </Button>
      )}
      {!change && requesting && (
        <>
          <label className="block space-y-1">
            <span>{t(($) => $.requirements_change.reason)}</span>
            <Textarea
              value={reason}
              onChange={(event) => setReason(event.target.value)}
            />
          </label>
          <Button
            size="sm"
            disabled={!reason.trim() || Array.from(reason).length > 2000}
            aria-busy={mutation.isPending}
            onClick={() => void act("start")}
          >
            {t(($) => $.requirements_change.request_stop)}
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setRequesting(false)}
          >
            {t(($) => $.requirements.discard)}
          </Button>
        </>
      )}
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
    </fieldset>
  );
}

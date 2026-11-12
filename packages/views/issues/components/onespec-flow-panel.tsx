"use client";
import { AppLink } from "../../navigation";
import { useWorkspacePaths } from "@multica/core/paths";
import { RequirementsPanel } from "./requirements-panel";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import type { Issue } from "@multica/core/types";
import {
  oneSpecFlowOptions,
  oneSpecKinds,
  oneSpecStages,
  useOneSpecAction,
  useSaveExecutionPlan,
  oneSpecConfigurationOptions,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { ConfigurationSelect } from "../../projects/components/configuration-select";
import { useT } from "../../i18n";

export function OneSpecFlowPanel({ issue }: { issue: Issue }) {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { t } = useT("projects");
  const saved = useQuery(oneSpecFlowOptions(wsId, issue.id));
  const project = useQuery({
    ...oneSpecConfigurationOptions(wsId, issue.project_id ?? ""),
    enabled: !!issue.project_id && !issue.parent_issue_id,
  });
  const action = useOneSpecAction(wsId, issue.id);
  const apply = useSaveExecutionPlan(wsId, issue.id);
  const [kind, setKind] = useState("feature");
  const [releaseEnvironment, setReleaseEnvironment] = useState("staging");
  const [reason, setReason] = useState("");
  const [reopenStage, setReopenStage] = useState("");
  const flow = saved.data?.flow;
  const labels: Record<string, string> = {
    branch: t(($) => $.onespec.branch),
    merging: t(($) => $.onespec.merging),
    feature: t(($) => $.onespec.feature),
    change: t(($) => $.onespec.change),
    bugfix: t(($) => $.onespec.bugfix),
    release: t(($) => $.onespec.release),
    documentation: t(($) => $.onespec.documentation),
    requirements: t(($) => $.onespec.requirements),
    design: t(($) => $.onespec.design),
    plan: t(($) => $.onespec.plan),
    implementation: t(($) => $.onespec.implementation),
    testing: t(($) => $.onespec.testing),
    archive: t(($) => $.onespec.archive),
    archiving: t(($) => $.onespec.archiving),
    acceptance: t(($) => $.onespec.acceptance),
    release_preparation: t(($) => $.onespec.release_preparation),
    deployment: t(($) => $.onespec.deployment),
    merge: t(($) => $.onespec.merge),
    complete: t(($) => $.onespec.complete),
    preparing: t(($) => $.onespec.preparing),
    awaiting_confirmation: t(($) => $.onespec.awaiting_confirmation),
    ready: t(($) => $.onespec.ready),
    completed: t(($) => $.onespec.completed),
  };
  if (issue.parent_issue_id || !issue.project_id) return null;
  if (!flow && !project.data?.enabled && !saved.isError) return null;
  const current = flow?.artifacts.findLast(
    (artifact) => artifact.stage === flow.phase,
  );
  const stages = flow ? oneSpecStages(flow.kind) : [];
  const reopenStages = stages.slice(
    0,
    flow?.phase === "complete"
      ? stages.length
      : stages.indexOf(flow?.phase ?? "") + 1,
  );
  const selectedReopenStage = reopenStages.includes(reopenStage)
    ? reopenStage
    : (reopenStages.at(-1) ?? "");
  const busy = action.isPending || apply.isPending;
  const send = (name: string, input: Record<string, unknown> = {}) =>
    action.mutate({
      action: name,
      input: { expected_revision: saved.data?.revision ?? 0, ...input },
    });
  return (
    <section
      className="space-y-3 rounded-lg border p-4"
      aria-label={t(($) => $.onespec.title)}
    >
      <h3 className="text-label font-medium">{t(($) => $.onespec.title)}</h3>
      {saved.isError && (
        <p role="alert">
          {t(($) => $.onespec.error)}{" "}
          <Button variant="ghost" onClick={() => void saved.refetch()}>
            {t(($) => $.onespec.retry)}
          </Button>
        </p>
      )}
      {!flow && !saved.isPending && (
        <>
          <ConfigurationSelect
            label={t(($) => $.onespec.kind)}
            value={kind}
            items={oneSpecKinds.map((value) => ({
              value,
              label: labels[value]!,
            }))}
            onChange={setKind}
          />
          <Button disabled={busy} onClick={() => send("start", { kind })}>
            {t(($) => $.onespec.start)}
          </Button>
        </>
      )}
      {flow && (
        <>
          <div
            className="flex flex-wrap gap-2 text-caption"
            aria-label={labels[flow.phase]}
          >
            {oneSpecStages(flow.kind).map((stage) => (
              <span
                key={stage}
                aria-current={stage === flow.phase ? "step" : undefined}
                className={
                  stage === flow.phase
                    ? "rounded bg-accent px-2 py-1 font-semibold"
                    : "px-2 py-1 text-muted-foreground"
                }
              >
                {labels[stage] ?? stage}
              </span>
            ))}
          </div>
          <p className="text-label">
            {labels[flow.phase] ?? flow.phase} ·{" "}
            {labels[flow.status] ?? flow.status}
          </p>
          <p className="text-caption text-muted-foreground">
            {t(($) => $.onespec.rule_version)} {flow.rule_version}
          </p>
          {flow.kind === "release" &&
            flow.phase === "release_preparation" &&
            !flow.release_id &&
            saved.data?.can_confirm && (
              <div className="space-y-2">
                <ConfigurationSelect
                  label={t(($) => $.releases.environment)}
                  value={releaseEnvironment}
                  onChange={setReleaseEnvironment}
                  disabled={busy}
                  items={[
                    { value: "staging", label: t(($) => $.releases.staging) },
                    { value: "prod", label: t(($) => $.releases.prod) },
                  ]}
                />
                <Button
                  disabled={busy}
                  onClick={() =>
                    send("release-draft", {
                      release: { env: releaseEnvironment },
                    })
                  }
                >
                  {t(($) => $.onespec.release_draft)}
                </Button>
              </div>
            )}
          {flow.release_id && (
            <AppLink
              href={paths.releaseDetail(flow.release_id)}
              className="text-body underline"
            >
              {t(($) => $.onespec.open_release)}
            </AppLink>
          )}
          {!flow.runtime_id && (
            <p role="status">{t(($) => $.onespec.missing_runtime)}</p>
          )}
          {flow.phase === "requirements" &&
            flow.artifacts.length === 0 &&
            saved.data?.can_confirm &&
            (issue.status === "todo" || issue.status === "backlog") && (
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => send("refresh-configuration")}
              >
                {t(($) => $.onespec.refresh_configuration)}
              </Button>
            )}
          {flow.phase === "requirements" &&
            flow.artifacts.length === 0 &&
            saved.data?.can_confirm && (
              <ConfigurationSelect
                label={t(($) => $.onespec.kind)}
                value={flow.kind}
                items={oneSpecKinds.map((value) => ({
                  value,
                  label: labels[value]!,
                }))}
                disabled={busy}
                onChange={(value) => send("kind", { kind: value })}
              />
            )}
          {current?.summary ? (
            <div className="space-y-2">
              <h4 className="text-label">
                {t(($) => $.onespec.artifacts)} · {t(($) => $.onespec.version)}{" "}
                {current.version}
              </h4>
              <p className="whitespace-pre-wrap break-words text-body">
                {current.summary}
              </p>
              <ul className="space-y-2">
                {current.refs.map((ref, index) => (
                  <li
                    key={index}
                    className="break-all rounded bg-muted p-2 text-caption"
                  >
                    <p>{ref.path}</p>
                    <p>{ref.repository_id}</p>
                    <p>
                      {t(($) => $.onespec.source)} {ref.commit}
                    </p>
                    <p>
                      {t(($) => $.onespec.digest)} {ref.digest}
                    </p>
                  </li>
                ))}
              </ul>
              {current.tasks.length > 0 && (
                <ol className="list-decimal space-y-1 pl-5">
                  {current.tasks.map((task) => (
                    <li key={task.key}>
                      {task.title} · {task.role}
                    </li>
                  ))}
                </ol>
              )}
            </div>
          ) : (
            <p className="text-caption text-muted-foreground">
              {t(($) => $.onespec.no_artifacts)}
            </p>
          )}
          {flow.status === "awaiting_confirmation" && (
            <>
              <p className="text-caption">{t(($) => $.onespec.confirm_hint)}</p>
              {saved.data?.can_confirm && current ? (
                <>
                  <Textarea
                    aria-label={t(($) => $.onespec.reason)}
                    placeholder={t(($) => $.onespec.reason)}
                    value={reason}
                    onChange={(event) => setReason(event.target.value)}
                    disabled={busy}
                  />
                  <div className="flex gap-2">
                    <Button
                      disabled={busy}
                      onClick={() =>
                        send("decision", {
                          stage: flow.phase,
                          version: current.version,
                          approved: true,
                        })
                      }
                    >
                      {t(($) => $.onespec.confirm)}
                    </Button>
                    <Button
                      variant="outline"
                      disabled={busy || !reason.trim()}
                      onClick={() =>
                        send("decision", {
                          stage: flow.phase,
                          version: current.version,
                          approved: false,
                          reason,
                        })
                      }
                    >
                      {t(($) => $.onespec.reject)}
                    </Button>
                  </div>
                </>
              ) : (
                <p role="status">
                  {t(($) => $.onespec.waiting)} · {flow.approver_id}
                </p>
              )}
            </>
          )}
          {flow.status === "ready" && saved.data?.can_confirm && (
            <div className="flex flex-wrap gap-2">
              {flow.phase === "plan" && current && (
                <Button
                  disabled={busy}
                  onClick={() =>
                    apply.mutate({
                      expected_revision: issue.revision ?? 0,
                      version: current.requirement_version,
                      idempotency_key: `onespec-${issue.id}-${current.version}`,
                      tasks: current.tasks,
                    })
                  }
                >
                  {t(($) => $.onespec.apply_plan)}
                </Button>
              )}
              <Button
                variant={flow.phase === "plan" ? "outline" : "default"}
                disabled={busy}
                onClick={() => send("advance")}
              >
                {t(($) => $.onespec.advance)}
              </Button>
            </div>
          )}
          {flow.status === "preparing" &&
            saved.data?.can_confirm &&
            flow.phase !== "acceptance" && (
              <Button
                disabled={busy || !flow.runtime_id}
                onClick={() => send("run")}
              >
                {t(($) => $.onespec.run)}
              </Button>
            )}
          {saved.data?.can_confirm && flow.artifacts.length > 0 && (
            <div className="space-y-2">
              <ConfigurationSelect
                label={t(($) => $.onespec.reopen_stage)}
                value={selectedReopenStage}
                items={reopenStages.map((value) => ({
                  value,
                  label: labels[value]!,
                }))}
                onChange={setReopenStage}
                disabled={busy}
              />
              <Button
                variant="ghost"
                disabled={busy}
                onClick={() => send("reopen", { stage: selectedReopenStage })}
              >
                {t(($) => $.onespec.reopen)}
              </Button>
            </div>
          )}
          {flow.decisions.length > 0 && (
            <details>
              <summary className="cursor-pointer text-caption">
                {t(($) => $.onespec.history)}
              </summary>
              <ul className="space-y-1 pt-2 text-caption">
                {flow.decisions.map((decision, index) => (
                  <li key={index}>
                    {labels[decision.stage]} · {t(($) => $.onespec.version)}{" "}
                    {decision.version} ·{" "}
                    {decision.approved
                      ? t(($) => $.onespec.confirm)
                      : t(($) => $.onespec.reject)}{" "}
                    · {decision.reason} · {decision.created_at}
                  </li>
                ))}
              </ul>
            </details>
          )}
          {flow.git_operations.length > 0 && (
            <details>
              <summary className="cursor-pointer text-caption">
                {t(($) => $.onespec.git_operations)}
              </summary>
              <ul className="space-y-1 pt-2 text-caption">
                {flow.git_operations.map((operation) => (
                  <li key={operation.key} className="break-all">
                    {operation.branch} · {operation.status} ·{" "}
                    {operation.summary}
                    <p>
                      {operation.base_commit} →{" "}
                      {operation.merged_commit || operation.published_commit}
                    </p>
                  </li>
                ))}
              </ul>
            </details>
          )}
        </>
      )}
      {(action.error || apply.error) && (
        <p role="alert" className="text-caption text-destructive">
          {action.error?.message ?? apply.error?.message}
        </p>
      )}
    </section>
  );
}

export function OneSpecRequirementsPanel({ issue }: { issue: Issue }) {
  const wsId = useWorkspaceId();
  const flow = useQuery(oneSpecFlowOptions(wsId, issue.id));
  return <RequirementsPanel issue={issue} oneSpecManaged={!!flow.data?.flow} />;
}

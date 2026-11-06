"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { projectResourcesOptions } from "@multica/core/projects";
import {
  projectRoleOrder,
  useSaveExecutionPlan,
  type ExecutionPlanTask,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";

export function ExecutionPlanEditor({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const { t: projectT } = useT("projects");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const resources = useQuery({
    ...projectResourcesOptions(wsId, issue.project_id ?? ""),
    enabled: !!issue.project_id,
  });
  const mutation = useSaveExecutionPlan(wsId, issue.id);
  const [draft, setDraft] = useState<{
    tasks: ExecutionPlanTask[];
    key: string;
    revision: number;
  }>();
  const [error, setError] = useState<string>();
  const criteria = issue.acceptance_criteria;
  if (
    !criteria?.gate.spec ||
    !criteria.team ||
    issue.parent_issue_id ||
    !issue.project_id ||
    issue.revision === undefined
  )
    return null;
  if (criteria.plan && !criteria.needs_resplit)
    return (
      <p className="text-caption text-muted-foreground">
        {t(($) => $.execution_plan.saved, {
          count: criteria.plan.issues.length,
          version: criteria.plan.version,
        })}
      </p>
    );
  if (
    !["owner", "admin", "member"].includes(member.role ?? "") ||
    !["todo", "backlog"].includes(issue.status)
  )
    return null;
  const enabledRoles = projectRoleOrder.filter(
    (role) => criteria.team!.config.roles[role].enabled,
  );
  const start = () => {
    setDraft({
      key: crypto.randomUUID(),
      revision: issue.revision!,
      tasks: [],
    });
    setError(undefined);
  };
  const change = (index: number, fields: Partial<ExecutionPlanTask>) => {
    if (!draft) return;
    setDraft({
      ...draft,
      tasks: draft.tasks.map((task, i) =>
        i === index ? { ...task, ...fields } : task,
      ),
    });
  };
  const save = async () => {
    if (!draft) return;
    setError(undefined);
    try {
      await mutation.mutateAsync({
        version: criteria.version,
        expected_revision: draft.revision,
        idempotency_key: draft.key,
        tasks: draft.tasks,
      });
      setDraft(undefined);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.requirements.save_failed),
      );
    }
  };
  if (!draft)
    return (
      <Button size="sm" variant="outline" onClick={start}>
        {t(($) => $.execution_plan.open)}
      </Button>
    );
  return (
    <fieldset disabled={mutation.isPending} className="space-y-3">
      <legend className="text-label font-medium">
        {t(($) => $.execution_plan.open)}
      </legend>
      {resources.isPending && (
        <p role="status">{t(($) => $.execution_plan.loading)}</p>
      )}
      {resources.isError && (
        <Button
          variant="outline"
          size="sm"
          onClick={() => void resources.refetch()}
        >
          {t(($) => $.execution_plan.retry)}
        </Button>
      )}
      {draft.tasks.map((task, index) => (
        <div key={task.key} className="space-y-2 rounded-xl border p-3">
          <div className="flex items-center justify-between">
            <span className="text-label">{task.key}</span>
            <Button
              size="xs"
              variant="ghost"
              onClick={() =>
                setDraft({
                  ...draft,
                  tasks: draft.tasks
                    .filter((_, i) => i !== index)
                    .map((entry) => ({
                      ...entry,
                      depends_on: entry.depends_on.filter(
                        (key) => key !== task.key,
                      ),
                    })),
                })
              }
            >
              {t(($) => $.requirements.remove)}
            </Button>
          </div>
          <label className="block text-caption">
            {t(($) => $.execution_plan.title)}
            <Input
              value={task.title}
              onChange={(e) => change(index, { title: e.target.value })}
            />
          </label>
          <label className="block text-caption">
            {t(($) => $.execution_plan.role)}
            <select
              className="block w-full rounded-md border bg-surface p-2"
              value={task.role}
              onChange={(e) => change(index, { role: e.target.value })}
            >
              {enabledRoles.map((role) => (
                <option key={role} value={role}>
                  {projectT(($) => $.roles[role])}
                </option>
              ))}
            </select>
          </label>
          <label className="block text-caption">
            {t(($) => $.execution_plan.repo)}
            <select
              className="block w-full rounded-md border bg-surface p-2"
              value={task.repo_id}
              onChange={(e) => change(index, { repo_id: e.target.value })}
            >
              <option value="">{t(($) => $.execution_plan.no_repo)}</option>
              {resources.data?.map((repo) => (
                <option key={repo.id} value={repo.id}>
                  {repo.label ?? repo.id}
                </option>
              ))}
            </select>
          </label>
          <fieldset className="space-y-1 text-caption">
            <legend>{t(($) => $.execution_plan.covers)}</legend>
            {criteria.items.map((item) => (
              <label key={item.id} className="flex items-start gap-2">
                <input
                  type="checkbox"
                  checked={task.covers.includes(item.id)}
                  onChange={(e) =>
                    change(index, {
                      covers: e.target.checked
                        ? [...task.covers, item.id]
                        : task.covers.filter((id) => id !== item.id),
                    })
                  }
                />
                {item.id} · {item.text}
              </label>
            ))}
          </fieldset>
          <fieldset className="space-y-1 text-caption">
            <legend>{t(($) => $.execution_plan.dependencies)}</legend>
            {draft.tasks
              .filter((entry) => entry.key !== task.key)
              .map((entry) => (
                <label key={entry.key} className="flex items-start gap-2">
                  <input
                    type="checkbox"
                    checked={task.depends_on.includes(entry.key)}
                    onChange={(e) =>
                      change(index, {
                        depends_on: e.target.checked
                          ? [...task.depends_on, entry.key]
                          : task.depends_on.filter((id) => id !== entry.key),
                      })
                    }
                  />
                  {entry.key} · {entry.title}
                </label>
              ))}
          </fieldset>
          <label className="block text-caption">
            {t(($) => $.execution_plan.serial_group)}
            <Input
              value={task.serial_group}
              onChange={(e) => change(index, { serial_group: e.target.value })}
            />
          </label>
          <label className="block text-caption">
            {t(($) => $.execution_plan.expected)}
            <Input
              value={task.expected.join(", ")}
              onChange={(e) =>
                change(index, {
                  expected: e.target.value
                    .split(",")
                    .map((value) => value.trim()),
                })
              }
            />
          </label>
        </div>
      ))}
      <div className="flex flex-wrap gap-2">
        <Button
          size="sm"
          variant="outline"
          disabled={draft.tasks.length >= 100}
          onClick={() =>
            setDraft({
              ...draft,
              tasks: [
                ...draft.tasks,
                {
                  key: crypto.randomUUID().slice(0, 8),
                  title: "",
                  description: "",
                  role: enabledRoles.includes("dev") ? "dev" : "product",
                  repo_id: "",
                  covers: [],
                  depends_on: [],
                  serial_group: "",
                  expected: ["branch", "diff", "selftest"],
                },
              ],
            })
          }
        >
          {t(($) => $.execution_plan.add)}
        </Button>
        <Button
          size="sm"
          disabled={
            !draft.tasks.length || resources.isPending || resources.isError
          }
          aria-busy={mutation.isPending}
          onClick={() => void save()}
        >
          {t(($) => $.execution_plan.save)}
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setDraft(undefined)}>
          {t(($) => $.requirements.discard)}
        </Button>
      </div>
      {error && (
        <p role="alert" className="text-caption text-destructive">
          {error}
        </p>
      )}
    </fieldset>
  );
}

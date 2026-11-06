"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { projectResourcesOptions } from "@multica/core/projects";
import { runtimeListOptions } from "@multica/core/runtimes";
import {
  credentialsOptions,
  deployTargetsOptions,
  deploymentCredentialKind,
  targetLocationFields,
  useDeleteDeployTarget,
  useSaveDeployTarget,
  type DeployTarget,
  type DeployTargetInput,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";
import { ConfigurationSelect } from "./configuration-select";

export function DeployTargetsSection({ projectId }: { projectId: string }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const { role } = useCurrentMember(wsId);
  const canManage = role === "owner" || role === "admin";
  const targets = useQuery(deployTargetsOptions(wsId, projectId));
  const resources = useQuery(projectResourcesOptions(wsId, projectId));
  const credentials = useQuery({
    ...credentialsOptions(wsId),
    enabled: canManage,
  });
  const runtimes = useQuery({
    ...runtimeListOptions(wsId),
    enabled: canManage,
  });
  const save = useSaveDeployTarget(wsId, projectId);
  const remove = useDeleteDeployTarget(wsId, projectId);
  const [editing, setEditing] = useState<{
    id?: string;
    data: DeployTargetInput;
  }>();
  const [deleting, setDeleting] = useState<DeployTarget>();
  const [error, setError] = useState<string>();
  const fields: Record<string, string> = {
    host: t(($) => $.workbench.host),
    image: t(($) => $.workbench.image),
    dir: t(($) => $.workbench.dir),
    domain: t(($) => $.workbench.domain),
    health_url: t(($) => $.workbench.health_url),
    start_command: t(($) => $.workbench.start_command),
    appid: t(($) => $.workbench.appid),
    robot: t(($) => $.workbench.robot),
    bucket: t(($) => $.workbench.bucket),
    registry: t(($) => $.workbench.registry),
  };
  const none = { value: "", label: t(($) => $.workbench.not_set) };
  const environments = ["preview", "staging", "prod"] as const;
  const change = (patch: Partial<DeployTargetInput>) =>
    setEditing(
      (current) =>
        current && { ...current, data: { ...current.data, ...patch } },
    );
  const start = (target?: DeployTarget) => {
    setError(undefined);
    setEditing(
      target
        ? {
            id: target.id,
            data: { ...target, expected_revision: target.revision },
          }
        : {
            data: {
              label: "",
              repo_id: resources.data?.[0]?.id ?? "",
              env: "staging",
              kind: "docker-host",
              channel: "runtime",
              runtime_id: null,
              credential_id: null,
              location: {},
            },
          },
    );
  };
  const submit = async () => {
    if (!editing) return;
    setError(undefined);
    try {
      await save.mutateAsync(editing);
      setEditing(undefined);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.workbench.load_failed),
      );
    }
  };
  const deleteTarget = async () => {
    if (!deleting) return;
    setError(undefined);
    try {
      await remove.mutateAsync(deleting);
      setDeleting(undefined);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.workbench.load_failed),
      );
    }
  };

  return (
    <section
      className="space-y-3 px-2"
      aria-label={t(($) => $.workbench.targets)}
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-label font-medium">
          {t(($) => $.workbench.targets)}
        </h3>
        {canManage && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => start()}
            disabled={!!editing}
          >
            {t(($) => $.workbench.add)}
          </Button>
        )}
      </div>
      {targets.isPending && <Skeleton className="h-16 w-full" />}
      {targets.isError && (
        <div role="alert" className="text-caption text-destructive">
          {t(($) => $.workbench.load_failed)}{" "}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void targets.refetch()}
          >
            {t(($) => $.workbench.retry)}
          </Button>
        </div>
      )}
      {targets.data?.targets.length === 0 && (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.workbench.empty)}
        </p>
      )}
      {targets.data?.targets.map((target) => (
        <div key={target.id} className="rounded-xl border bg-surface p-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{target.label}</span>
            <span className="text-caption text-muted-foreground">
              {target.env} · {target.kind}
            </span>
            {canManage && (
              <>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => start(target)}
                  disabled={!!editing}
                >
                  {t(($) => $.workbench.edit)}
                </Button>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => setDeleting(target)}
                >
                  {t(($) => $.workbench.remove)}
                </Button>
              </>
            )}
          </div>
          <p className="text-caption text-warning">
            {target.missing_fields.length
              ? t(($) => $.workbench.incomplete)
              : t(($) => $.workbench.untested)}
          </p>
          <p className="break-words text-caption text-muted-foreground">
            {target.location.domain ||
              target.location.host ||
              target.location.registry}
          </p>
        </div>
      ))}
      {deleting && (
        <div className="space-y-2 rounded-xl border p-3">
          <p>
            {t(($) => $.workbench.delete_confirm)} {deleting.label}
          </p>
          <Button
            variant="destructive"
            disabled={remove.isPending}
            onClick={() => void deleteTarget()}
          >
            {t(($) => $.workbench.remove)}
          </Button>
          <Button
            variant="ghost"
            disabled={remove.isPending}
            onClick={() => setDeleting(undefined)}
          >
            {t(($) => $.workbench.cancel)}
          </Button>
        </div>
      )}
      {editing && (
        <form
          className="space-y-3 rounded-xl border bg-surface p-3"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <fieldset className="space-y-3" disabled={save.isPending}>
            <label className="block space-y-1 text-caption">
              {t(($) => $.workbench.name)}
              <Input
                required
                maxLength={120}
                value={editing.data.label}
                onChange={(event) => change({ label: event.target.value })}
              />
            </label>
            <div className="grid gap-3 sm:grid-cols-2">
              <ConfigurationSelect
                label={t(($) => $.workbench.repository)}
                value={editing.data.repo_id}
                items={[
                  none,
                  ...(resources.data ?? []).map((repo) => ({
                    value: repo.id,
                    label: repo.label || repo.resource_type,
                  })),
                ]}
                onChange={(repo_id) => change({ repo_id })}
              />
              <ConfigurationSelect
                label={t(($) => $.workbench.environment)}
                value={editing.data.env}
                items={environments.map((env) => ({
                  value: env,
                  label: t(($) => $.workbench[env]),
                }))}
                onChange={(env) => change({ env })}
              />
              <ConfigurationSelect
                label={t(($) => $.workbench.kind)}
                value={editing.data.kind}
                items={Object.keys(targetLocationFields).map((kind) => ({
                  value: kind,
                  label: kind,
                }))}
                onChange={(kind) =>
                  change({
                    kind,
                    location: {},
                    channel:
                      kind === "docker-host" || kind === "service-process"
                        ? "runtime"
                        : "platform",
                    runtime_id: null,
                    credential_id: null,
                  })
                }
              />
              <ConfigurationSelect
                label={t(($) => $.workbench.channel)}
                value={editing.data.channel}
                items={(editing.data.kind === "docker-host" ||
                editing.data.kind === "service-process"
                  ? ["runtime", "ssh"]
                  : ["platform"]
                ).map((channel) => ({ value: channel, label: channel }))}
                onChange={(channel) =>
                  change({ channel, runtime_id: null, credential_id: null })
                }
              />
              {editing.data.channel === "runtime" && (
                <ConfigurationSelect
                  label={t(($) => $.workbench.runtime)}
                  value={editing.data.runtime_id ?? ""}
                  items={[
                    none,
                    ...(runtimes.data ?? []).map((runtime) => ({
                      value: runtime.id,
                      label: runtime.name,
                    })),
                  ]}
                  onChange={(runtime_id) =>
                    change({ runtime_id: runtime_id || null })
                  }
                />
              )}
              <ConfigurationSelect
                label={t(($) => $.workbench.credential)}
                value={editing.data.credential_id ?? ""}
                items={[
                  none,
                  ...(credentials.data?.credentials ?? [])
                    .filter(
                      (credential) =>
                        credential.usage.includes("deploy") &&
                        (!deploymentCredentialKind(
                          editing.data.kind,
                          editing.data.channel,
                        ) ||
                          credential.kind ===
                            deploymentCredentialKind(
                              editing.data.kind,
                              editing.data.channel,
                            )) &&
                        (!credential.project_id ||
                          credential.project_id === projectId),
                    )
                    .map((credential) => ({
                      value: credential.id,
                      label: `${credential.label} · ••••${credential.last_four}`,
                    })),
                ]}
                onChange={(credential_id) =>
                  change({ credential_id: credential_id || null })
                }
              />
            </div>
            {(targetLocationFields[editing.data.kind] ?? []).map((field) => (
              <label key={field} className="block space-y-1 text-caption">
                {fields[field]}
                <Input
                  maxLength={2048}
                  value={editing.data.location[field] ?? ""}
                  onChange={(event) =>
                    change({
                      location: {
                        ...editing.data.location,
                        [field]: event.target.value,
                      },
                    })
                  }
                />
              </label>
            ))}
            <div className="flex gap-2">
              <Button
                type="submit"
                disabled={save.isPending || !editing.data.repo_id}
                aria-busy={save.isPending}
              >
                {t(($) => $.workbench.save)}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={save.isPending}
                onClick={() => setEditing(undefined)}
              >
                {t(($) => $.workbench.cancel)}
              </Button>
            </div>
          </fieldset>
        </form>
      )}
      {error && (
        <p role="alert" className="text-caption text-destructive">
          {error}
        </p>
      )}
    </section>
  );
}

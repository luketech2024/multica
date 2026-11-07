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
  deploymentCheckStatus,
  useCheckDeployTarget,
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
  const check = useCheckDeployTarget(wsId, projectId);
  const [environment, setEnvironment] = useState("staging");
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
  const start = (target?: DeployTarget, repoId?: string) => {
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
              repo_id: repoId ?? resources.data?.[0]?.id ?? "",
              env: environment,
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
      const saved = await save.mutateAsync(editing);
      setEditing(undefined);
      if (saved.channel === "runtime" && saved.missing_fields.length === 0) {
        check.mutate({ id: saved.id, revision: saved.revision });
      }
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

  const repositories = (resources.data ?? []).map((resource) => {
    const ref = resource.resource_ref;
    const location = "url" in ref ? ref.url : ref.local_path;
    return { ...resource, label: resource.label || (typeof location === "string" ? location : resource.id) };
  });
  const visibleTargets = (targets.data?.targets ?? []).filter((target) => target.env === environment);
  const unboundRepositories = repositories.filter((repo) => !visibleTargets.some((target) => target.repo_id === repo.id));

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
      <ConfigurationSelect
        label={t(($) => $.workbench.environment)}
        value={environment}
        items={environments.map((env) => ({ value: env, label: t(($) => $.workbench[env]) }))}
        onChange={setEnvironment}
        disabled={!!editing}
      />
      {targets.data && (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.deployment.summary, {
            repositories: repositories.length,
            targets: visibleTargets.length,
            pending: visibleTargets.filter((target) => deploymentCheckStatus(target) !== "passed").length,
          })}
        </p>
      )}
      {(targets.isPending || resources.isPending) && <Skeleton className="h-16 w-full" />}
      {(targets.isError || resources.isError) && (
        <div role="alert" className="text-caption text-destructive">
          {t(($) => $.workbench.load_failed)}{" "}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => { void targets.refetch(); void resources.refetch(); }}
          >
            {t(($) => $.workbench.retry)}
          </Button>
        </div>
      )}
      {targets.data && visibleTargets.length === 0 && (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.workbench.empty)}
        </p>
      )}
      {check.isError && (
        <p role="alert" className="text-caption text-destructive">
          {check.error.message}
        </p>
      )}
      {environment !== "preview" && targets.data && unboundRepositories.map((repo) => (
        <div key={repo.id} className="space-y-2 rounded-xl border border-dashed p-3">
          <p className="font-mono text-caption">{repo.label}</p>
          <p className="text-caption text-muted-foreground">{t(($) => $.deployment.unbound)}</p>
          {canManage && <Button variant="outline" size="sm" disabled={!!editing} onClick={() => start(undefined, repo.id)}>
            {t(($) => $.deployment.bind)}
          </Button>}
        </div>
      ))}
      {visibleTargets.map((target) => (
        <div key={target.id} className="rounded-xl border bg-surface p-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{target.label}</span>
            <span className="text-caption text-muted-foreground">
              {target.env} · {target.kind}
            </span>
            {canManage && (
              <>
                {target.channel === "runtime" && (
                  <Button
                    variant="outline"
                    size="xs"
                    disabled={check.isPending || target.missing_fields.length > 0}
                    onClick={() => check.mutate({ id: target.id, revision: target.revision })}
                  >
                    {t(($) => $.workbench.check_target)}
                  </Button>
                )}
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
          <p className="font-mono text-caption">
            {repositories.find((repo) => repo.id === target.repo_id)?.label || target.repo_id}
          </p>
          <p className={deploymentCheckStatus(target) === "passed"
            ? "text-caption text-success"
            : "text-caption text-warning"}>
            {t(($) => $.workbench.check_states[deploymentCheckStatus(target)])}
          </p>
          {target.last_check && (
            <p className="break-words text-caption text-muted-foreground">
              {target.last_check.detail} · {new Date(target.last_check.at).toLocaleString()} · {t(($) => $.deployment.duration, { ms: target.last_check.ms })}
            </p>
          )}
          {target.last_check?.free_bytes != null && (
            <p className="text-caption text-muted-foreground">
              {t(($) => $.deployment.disk_free, { mb: Math.floor(target.last_check.free_bytes / (1024 * 1024)).toLocaleString() })}
            </p>
          )}
          <dl className="grid grid-cols-[minmax(0,1fr)_minmax(0,2fr)] gap-x-3 gap-y-1 text-caption">
            <dt className="text-muted-foreground">{t(($) => $.workbench.channel)}</dt>
            <dd className="break-words">{target.channel}</dd>
            {Object.entries(target.location).filter(([, value]) => value).map(([field, value]) => (
              <div key={field} className="contents">
                <dt className="text-muted-foreground">{fields[field] || field}</dt>
                <dd className="break-words">{value}</dd>
              </div>
            ))}
            {canManage && target.credential_id && <>
              <dt className="text-muted-foreground">{t(($) => $.workbench.credential)}</dt>
              <dd className="break-words">{(() => {
                const credential = credentials.data?.credentials.find((item) => item.id === target.credential_id);
                return credential ? `${credential.label} · ••••${credential.last_four}` : t(($) => $.workbench.not_set);
              })()}</dd>
            </>}
          </dl>
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
                  ...repositories.map((repo) => ({
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

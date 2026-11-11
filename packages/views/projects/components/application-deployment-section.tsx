"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { useWorkspacePaths } from "@multica/core/paths";
import { projectResourcesOptions } from "@multica/core/projects";
import {
  projectDeploymentOptions,
  useSaveProjectDeployment,
  type DeploymentApplication,
  type ProjectDeployment,
  type DeploymentResource,
} from "@multica/core/workbench/application-deployment";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { ConfigurationSelect } from "./configuration-select";
import { ApplicationDeploymentWizard } from "./application-deployment-wizard";
import {
  DeploymentResourceDialog,
  DeploymentResourceCheck,
} from "./deployment-resource-fields";

export function ApplicationDeploymentSection({
  projectId,
  view = "applications",
}: {
  projectId: string;
  view?: "applications" | "deployment";
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { role } = useCurrentMember(wsId);
  const canManage = role === "owner" || role === "admin";
  const resultStatuses: Record<string, string> = t(
    ($) => $.application_deployment.result_status,
    { returnObjects: true },
  );
  const query = useQuery(projectDeploymentOptions(wsId, projectId));
  const [environment, setEnvironment] = useState<"staging" | "prod">("staging");
  const [editing, setEditing] = useState<DeploymentApplication>();
  const [configuring, setConfiguring] = useState<string>();
  const [resource, setResource] = useState<DeploymentResource>();
  const save = useSaveProjectDeployment(wsId, projectId);
  const apps = query.data?.config.applications ?? [];
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        {view === "deployment" ? (
          <ConfigurationSelect
            label={t(($) => $.releases.environment)}
            value={environment}
            onChange={(value) =>
              setEnvironment(value === "prod" ? "prod" : "staging")
            }
            items={[
              { value: "staging", label: t(($) => $.releases.staging) },
              { value: "prod", label: t(($) => $.releases.prod) },
            ]}
          />
        ) : (
          <h2 className="text-title-sm font-medium">
            {t(($) => $.application_deployment.applications)}
          </h2>
        )}
        {canManage && query.data && (
          <Button
            onClick={() =>
              setEditing({
                id: crypto.randomUUID(),
                name: "",
                source_kind: "git",
                source: "",
                repo_id: "",
                artifact_path: ".",
                build_command: "",
                environments: {},
              })
            }
          >
            {t(($) => $.application_deployment.add_application)}
          </Button>
        )}
      </div>
      {query.isPending && <p role="status">{t(($) => $.releases.loading)}</p>}
      {view === "applications" && query.data && canManage && (
        <ConfigurationSelect
          label={t(($) => $.application_deployment.default_mode)}
          value={query.data.config.mode}
          disabled={save.isPending}
          onChange={(value) =>
            save.mutate({
              ...query.data!,
              config: {
                ...query.data!.config,
                mode: value === "auto" ? "auto" : "orchestrated",
              },
            })
          }
          items={[
            { value: "auto", label: t(($) => $.application_deployment.auto) },
            {
              value: "orchestrated",
              label: t(($) => $.application_deployment.orchestrated),
            },
          ]}
        />
      )}
      {save.isError && (
        <p role="alert" className="text-destructive">
          {save.error.message}
        </p>
      )}
      {query.isError && (
        <div role="alert">
          <p>{query.error.message}</p>
          <Button variant="outline" onClick={() => void query.refetch()}>
            {t(($) => $.releases.retry)}
          </Button>
        </div>
      )}
      {query.isSuccess && !apps.length && (
        <p className="rounded-xl border p-6 text-muted-foreground">
          {t(($) => $.application_deployment.empty)}
        </p>
      )}
      <div className="grid gap-4 xl:grid-cols-2">
        {apps.map((app) => {
          const config = app.environments[environment];
          return (
            <article
              key={app.id}
              className="space-y-3 rounded-xl border bg-surface p-4"
            >
              <h3 className="text-title-sm font-medium">{app.name}</h3>
              <p className="break-all text-caption text-muted-foreground">
                {app.source ||
                  t(($) => $.application_deployment.source_missing)}
              </p>
              {view === "deployment" && (
                <>
                  <p>
                    {config
                      ? `${config.kind} · ${config.nodes.length} ${t(($) => $.application_deployment.nodes)}`
                      : t(($) => $.application_deployment.unconfigured)}
                  </p>
                  {config?.domain && <p>{config.domain}</p>}
                  {(query.data?.deployments ?? [])
                    .filter(
                      (result) =>
                        result.application_id === app.id &&
                        result.env === environment,
                    )
                    .map((result) => (
                      <p
                        key={result.node_id}
                        className="break-all text-caption"
                      >
                        <AppLink
                          className="text-brand underline"
                          href={paths.releaseDetail(result.release_id)}
                        >
                          {t(($) => $.application_deployment.latest_release)}
                        </AppLink>
                        {" · "}
                        {
                          query.data?.config.resources.find(
                            (resource) => resource.id === result.node_id,
                          )?.name
                        }
                        {" · "}
                        {result.version}
                        {" · "}
                        {resultStatuses[result.status] ?? result.status}
                      </p>
                    ))}
                </>
              )}
              <div className="flex flex-wrap gap-2">
                {canManage && (
                  <>
                    <Button variant="outline" onClick={() => setEditing(app)}>
                      {t(($) => $.application_deployment.edit_application)}
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setConfiguring(app.id)}
                    >
                      {t(($) => $.application_deployment.configure)}
                    </Button>
                  </>
                )}
                <AppLink
                  className="inline-flex items-center rounded-lg px-3 text-label text-brand underline"
                  href={`${paths.releases()}?project=${projectId}&env=${environment}&application=${app.id}`}
                >
                  {t(($) => $.releases.create)}
                </AppLink>
              </div>
            </article>
          );
        })}
      </div>
      {view === "deployment" && query.data && (
        <section className="space-y-3 rounded-xl border p-4">
          <div className="flex justify-between">
            <h2 className="text-title-sm font-medium">
              {t(($) => $.application_deployment.resources)}
            </h2>
            {canManage && (
              <Button
                variant="outline"
                onClick={() =>
                  setResource({
                    id: crypto.randomUUID(),
                    name: "",
                    env: environment,
                    host: "",
                    port: "22",
                    username: "deploy",
                    host_key: "",
                    credential_id: "",
                  })
                }
              >
                {t(($) => $.application_deployment.add_server)}
              </Button>
            )}
          </div>
          {query.data.config.resources
            .filter((item) => item.env === environment)
            .map((item) => (
              <div
                key={item.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded-lg border p-3"
              >
                <div>
                  {item.name}
                  <p className="text-caption text-muted-foreground">
                    {item.host}
                  </p>
                </div>
                {canManage && (
                  <div className="flex flex-wrap gap-2">
                    <DeploymentResourceCheck
                      key={`${item.id}-${query.data.revision}`}
                      projectId={projectId}
                      resourceId={item.id}
                    />
                    <Button variant="outline" onClick={() => setResource(item)}>
                      {t(($) => $.application_deployment.edit_resource)}
                    </Button>
                  </div>
                )}
              </div>
            ))}
        </section>
      )}
      {resource && query.data && (
        <DeploymentResourceDialog
          projectId={projectId}
          initial={resource}
          data={query.data}
          onClose={() => setResource(undefined)}
        />
      )}
      {editing && query.data && (
        <ApplicationEditor
          key={editing.id}
          projectId={projectId}
          initial={editing}
          data={query.data}
          onClose={() => setEditing(undefined)}
        />
      )}
      {configuring && query.data && (
        <ApplicationDeploymentWizard
          projectId={projectId}
          applicationId={configuring}
          environment={environment}
          initial={query.data}
          onClose={() => setConfiguring(undefined)}
        />
      )}
    </section>
  );
}

function ApplicationEditor({
  projectId,
  initial,
  data,
  onClose,
}: {
  projectId: string;
  initial: DeploymentApplication;
  data: ProjectDeployment;
  onClose: () => void;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const resources = useQuery(projectResourcesOptions(wsId, projectId));
  const save = useSaveProjectDeployment(wsId, projectId);
  const [app, setApp] = useState(initial);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    const applications = data.config.applications.filter(
      (entry) => entry.id !== app.id,
    );
    applications.push(app);
    try {
      await save.mutateAsync({
        ...data,
        config: { ...data.config, applications },
      });
      onClose();
    } catch {
      return;
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) onClose();
      }}
    >
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>
            {t(($) => $.application_deployment.edit_application)}
          </DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <label className="block space-y-1">
            {t(($) => $.application_deployment.name)}
            <Input
              required
              maxLength={120}
              value={app.name}
              onChange={(e) => setApp({ ...app, name: e.target.value })}
            />
          </label>
          <ConfigurationSelect
            label={t(($) => $.application_deployment.source_kind)}
            value={app.source_kind}
            onChange={(value) =>
              setApp({
                ...app,
                source_kind: value as DeploymentApplication["source_kind"],
              })
            }
            items={[
              { value: "git", label: t(($) => $.application_deployment.git) },
              {
                value: "image",
                label: t(($) => $.application_deployment.image),
              },
              {
                value: "directory",
                label: t(($) => $.application_deployment.directory_source),
              },
            ]}
          />
          {app.source_kind === "git" && (
            <ConfigurationSelect
              label={t(($) => $.application_deployment.repository)}
              value={app.repo_id}
              onChange={(id) => {
                const repo = resources.data?.find((item) => item.id === id);
                const ref = repo?.resource_ref;
                setApp({
                  ...app,
                  repo_id: id,
                  source:
                    ref && "url" in ref && typeof ref.url === "string"
                      ? ref.url
                      : app.source,
                });
              }}
              items={[
                {
                  value: "",
                  label: t(($) => $.application_deployment.manual_source),
                },
                ...(resources.data ?? []).map((repo) => ({
                  value: repo.id,
                  label: repo.label || repo.id,
                })),
              ]}
            />
          )}
          <label className="block space-y-1">
            {t(($) => $.application_deployment.source)}
            <Input
              required
              value={app.source}
              onChange={(e) => setApp({ ...app, source: e.target.value })}
            />
          </label>
          {app.source_kind !== "image" && (
            <label className="block space-y-1">
              {t(($) => $.application_deployment.artifact_path)}
              <Input
                value={app.artifact_path}
                onChange={(e) =>
                  setApp({ ...app, artifact_path: e.target.value })
                }
              />
            </label>
          )}
          {app.source_kind === "git" && (
            <details>
              <summary>{t(($) => $.application_deployment.advanced)}</summary>
              <label className="mt-3 block space-y-1">
                {t(($) => $.application_deployment.build_command)}
                <Input
                  value={app.build_command}
                  onChange={(e) =>
                    setApp({ ...app, build_command: e.target.value })
                  }
                />
              </label>
            </details>
          )}
          {save.isError && (
            <p role="alert" className="text-destructive">
              {save.error.message}
            </p>
          )}
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={save.isPending}
              onClick={onClose}
            >
              {t(($) => $.releases.cancel)}
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {t(($) => $.application_deployment.save)}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

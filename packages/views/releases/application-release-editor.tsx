"use client";

import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useCurrentMember } from "@multica/core/permissions";
import { projectListOptions } from "@multica/core/projects";
import {
  projectDeploymentOptions,
  useUpdateApplicationRelease,
  useExecuteApplicationRelease,
  type ApplicationRelease,
  type ReleaseChange,
} from "@multica/core/workbench/application-deployment";
import {
  useCreateReleaseOrder,
  type ReleaseOrder,
} from "@multica/core/workbench/releases";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../i18n";
import { useNavigation, AppLink } from "../navigation";
import { ConfigurationSelect } from "../projects/components/configuration-select";
import { ApplicationDeploymentWizard } from "../projects/components/application-deployment-wizard";
import { ReleaseGates } from "./release-gates";

export function CreateApplicationReleaseDialog({
  initialProject,
}: {
  initialProject: string;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const [open, setOpen] = useState(false);
  const [project, setProject] = useState(initialProject);
  const [env, setEnv] = useState<"staging" | "prod">(
    navigation.searchParams.get("env") === "prod" ? "prod" : "staging",
  );
  const [title, setTitle] = useState("");
  const projects = useQuery(projectListOptions(wsId));
  const create = useCreateReleaseOrder(wsId);
  const deployment = useQuery(projectDeploymentOptions(wsId, project));
  const attempt = useRef<{ payload: string; id: string }>(undefined);
  const initialChangeId = useRef(crypto.randomUUID());
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    const selected = deployment.data?.config.applications.find(
      (app) => app.id === navigation.searchParams.get("application"),
    );
    const changes: ReleaseChange[] = selected
      ? [
          {
            id: initialChangeId.current,
            type: "app",
            name: selected.name,
            application_id: selected.id,
            version: "",
            after: "",
            body: "",
            path: "",
            database: "",
            rehearsed: false,
          },
        ]
      : [];
    const input = {
      project_id: project,
      env,
      tasks: [],
      delivery: {
        title,
        mode: deployment.data?.config.mode ?? "auto",
        changes,
        results: [],
      },
    };
    const payload = JSON.stringify(input);
    if (attempt.current?.payload !== payload)
      attempt.current = { payload, id: crypto.randomUUID() };
    try {
      const order = await create.mutateAsync({
        ...input,
        request_id: attempt.current.id,
      });
      setOpen(false);
      navigation.push(paths.releaseDetail(order.id));
    } catch {
      return;
    }
  };
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!create.isPending) setOpen(value);
      }}
    >
      <DialogTrigger render={<Button />}>
        {t(($) => $.application_deployment.existing_release)}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t(($) => $.releases.create)}</DialogTitle>
        </DialogHeader>
        <form className="space-y-4" onSubmit={submit}>
          <ConfigurationSelect
            label={t(($) => $.releases.project)}
            value={project}
            onChange={setProject}
            items={[
              { value: "", label: t(($) => $.releases.choose_project) },
              ...(projects.data ?? []).map((item) => ({
                value: item.id,
                label: item.title,
              })),
            ]}
          />
          <ConfigurationSelect
            label={t(($) => $.releases.environment)}
            value={env}
            onChange={(value) => setEnv(value === "prod" ? "prod" : "staging")}
            items={(["staging", "prod"] as const).map((value) => ({
              value,
              label: t(($) => $.releases[value]),
            }))}
          />
          <label className="block space-y-1">
            {t(($) => $.application_deployment.title)}
            <Input
              required
              maxLength={120}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </label>
          {create.isError && (
            <p role="alert" className="text-destructive">
              {create.error.message}
            </p>
          )}
          <div className="flex justify-end">
            <Button
              type="submit"
              disabled={!project || !title.trim() || create.isPending}
            >
              {t(($) => $.releases.create)}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function ApplicationReleaseEditor({ order }: { order: ReleaseOrder }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { role } = useCurrentMember(wsId);
  const query = useQuery(projectDeploymentOptions(wsId, order.project_id));
  const save = useUpdateApplicationRelease(wsId, order.id);
  const [draft, setDraft] = useState<ApplicationRelease>(
    order.manifest.delivery!,
  );
  const [step, setStep] = useState(0);
  const [configuring, setConfiguring] = useState<string>();
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const canManage = role === "owner" || role === "admin";
  const apps = query.data?.config.applications ?? [];
  const update = (patch: Partial<ApplicationRelease>) => {
    setDraft({ ...draft, ...patch });
    setSaved(false);
  };
  const change = (id: string, patch: Partial<ReleaseChange>) =>
    update({
      changes: draft.changes.map((item) =>
        item.id === id
          ? {
              ...item,
              ...patch,
              ...("body" in patch || "database" in patch || "version" in patch
                ? { rehearsed: false }
                : {}),
            }
          : item,
      ),
    });
  const add = (type: ReleaseChange["type"], applicationId = apps[0]?.id) => {
    if (!applicationId) return;
    const app = apps.find((item) => item.id === applicationId)!;
    update({
      changes: [
        ...draft.changes,
        {
          id: crypto.randomUUID(),
          type,
          name: app.name,
          application_id: app.id,
          version: "",
          after: draft.changes.at(-1)?.id ?? "",
          body: "",
          path: "",
          database: "",
          rehearsed: false,
        },
      ],
    });
  };
  const persist = async (next?: number) => {
    setError("");
    try {
      await save.mutateAsync({
        delivery: draft,
        expected_revision: order.revision,
      });
      setSaved(true);
      if (next !== undefined) setStep(next);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };
  const move = (index: number, direction: number) => {
    const changes = [...draft.changes];
    const other = index + direction;
    if (other < 0 || other >= changes.length) return;
    [changes[index], changes[other]] = [changes[other]!, changes[index]!];
    const positions = new Map(changes.map((item, i) => [item.id, i]));
    update({
      changes: changes.map((item, i) =>
        item.after && (positions.get(item.after) ?? i) >= i
          ? { ...item, after: "" }
          : item,
      ),
    });
  };
  if (order.status !== "draft")
    return <ApplicationReleaseResults order={order} />;
  return (
    <section className="space-y-4">
      <ol className="flex flex-wrap gap-3 text-label">
        {[
          t(($) => $.application_deployment.content),
          t(($) => $.application_deployment.order),
          t(($) => $.application_deployment.review),
        ].map((label, index) => (
          <li
            key={label}
            aria-current={step === index ? "step" : undefined}
            className={
              step === index
                ? "font-medium text-brand"
                : "text-muted-foreground"
            }
          >
            {index + 1} · {label}
          </li>
        ))}
      </ol>
      {query.isPending && <p role="status">{t(($) => $.releases.loading)}</p>}
      {query.isError && <p role="alert">{query.error.message}</p>}
      {step === 0 && (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block space-y-1">
              {t(($) => $.application_deployment.title)}
              <Input
                value={draft.title}
                onChange={(e) => update({ title: e.target.value })}
              />
            </label>
            <ConfigurationSelect
              label={t(($) => $.application_deployment.mode)}
              value={draft.mode}
              onChange={(value) => {
                if (
                  value === "auto" &&
                  draft.changes.some((item) => item.type !== "app")
                ) {
                  setError(t(($) => $.application_deployment.mode_conflict));
                  return;
                }
                update({ mode: value === "auto" ? "auto" : "orchestrated" });
              }}
              items={[
                {
                  value: "auto",
                  label: t(($) => $.application_deployment.auto),
                },
                {
                  value: "orchestrated",
                  label: t(($) => $.application_deployment.orchestrated),
                },
              ]}
            />
          </div>
          <div className="flex flex-wrap gap-2">
            {apps
              .filter(
                (app) =>
                  !draft.changes.some(
                    (item) =>
                      item.type === "app" && item.application_id === app.id,
                  ),
              )
              .map((app) => (
                <Button
                  key={app.id}
                  variant="outline"
                  onClick={() => add("app", app.id)}
                >
                  {t(($) => $.application_deployment.add_application)} ·{" "}
                  {app.name}
                </Button>
              ))}
            {draft.mode === "orchestrated" && (
              <>
                <Button
                  variant="outline"
                  disabled={!apps.length}
                  onClick={() => add("config")}
                >
                  {t(($) => $.application_deployment.add_config)}
                </Button>
                <Button
                  variant="outline"
                  disabled={!apps.length}
                  onClick={() => add("sql")}
                >
                  {t(($) => $.application_deployment.add_sql)}
                </Button>
              </>
            )}
          </div>
          {!apps.length && query.isSuccess && (
            <AppLink
              className="text-brand underline"
              href={`${paths.projectDetail(order.project_id)}?tab=applications`}
            >
              {t(($) => $.application_deployment.add_application)}
            </AppLink>
          )}
          {draft.changes.map((item) => (
            <article
              key={item.id}
              className="space-y-3 rounded-xl border bg-surface p-4"
            >
              <div className="flex items-center justify-between">
                <h3 className="font-medium">
                  {item.type === "app"
                    ? t(($) => $.application_deployment.application)
                    : item.type === "sql"
                      ? "SQL"
                      : t(($) => $.application_deployment.add_config)}{" "}
                  · {item.name}
                </h3>
                <Button
                  variant="ghost"
                  onClick={() =>
                    update({
                      changes: draft.changes
                        .filter((entry) => entry.id !== item.id)
                        .map((entry) =>
                          entry.after === item.id
                            ? { ...entry, after: item.after }
                            : entry,
                        ),
                    })
                  }
                >
                  {t(($) => $.application_deployment.remove)}
                </Button>
              </div>
              {item.type !== "app" && (
                <ConfigurationSelect
                  label={t(($) => $.application_deployment.application)}
                  value={item.application_id}
                  onChange={(id) =>
                    change(item.id, {
                      application_id: id,
                      name: apps.find((app) => app.id === id)?.name ?? "",
                    })
                  }
                  items={apps.map((app) => ({
                    value: app.id,
                    label: app.name,
                  }))}
                />
              )}
              {(item.type === "app" || item.type === "sql") && (
                <label className="block space-y-1">
                  {t(($) => $.application_deployment.version)}
                  <Input
                    value={item.version}
                    onChange={(e) =>
                      change(item.id, { version: e.target.value })
                    }
                  />
                </label>
              )}
              {item.type === "config" && (
                <label className="block space-y-1">
                  {t(($) => $.application_deployment.target_path)}
                  <Input
                    value={item.path}
                    onChange={(e) => change(item.id, { path: e.target.value })}
                  />
                </label>
              )}
              {item.type === "sql" && (
                <label className="block space-y-1">
                  {t(($) => $.application_deployment.database)}
                  <Input
                    value={item.database}
                    onChange={(e) =>
                      change(item.id, { database: e.target.value })
                    }
                  />
                  <span className="text-caption text-muted-foreground">
                    {t(($) => $.application_deployment.database_hint)}
                  </span>
                </label>
              )}
              {item.type !== "app" && (
                <label className="block space-y-1">
                  {t(($) => $.application_deployment.body)}
                  <Textarea
                    rows={5}
                    value={item.body}
                    onChange={(e) => change(item.id, { body: e.target.value })}
                  />
                </label>
              )}
              {item.type === "sql" && (
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={item.rehearsed}
                    onChange={(e) =>
                      change(item.id, { rehearsed: e.target.checked })
                    }
                  />
                  {t(($) => $.application_deployment.rehearsed)}
                </label>
              )}
              {canManage && (
                <Button
                  variant="outline"
                  onClick={() => setConfiguring(item.application_id)}
                >
                  {t(($) => $.application_deployment.configure)}
                </Button>
              )}
            </article>
          ))}
        </>
      )}
      {step === 1 &&
        draft.changes.map((item, index) => (
          <article key={item.id} className="space-y-3 rounded-xl border p-4">
            <div className="flex justify-between">
              <span>
                {index + 1} · {item.name} · {item.type}
              </span>
              <div>
                <Button
                  variant="ghost"
                  disabled={index === 0}
                  onClick={() => move(index, -1)}
                >
                  {t(($) => $.application_deployment.up)}
                </Button>
                <Button
                  variant="ghost"
                  disabled={index === draft.changes.length - 1}
                  onClick={() => move(index, 1)}
                >
                  {t(($) => $.application_deployment.down)}
                </Button>
              </div>
            </div>
            <ConfigurationSelect
              label={t(($) => $.application_deployment.after)}
              value={item.after}
              onChange={(after) => change(item.id, { after })}
              items={[
                {
                  value: "",
                  label: t(($) => $.application_deployment.independent),
                },
                ...draft.changes.slice(0, index).map((entry) => ({
                  value: entry.id,
                  label: entry.name + " · " + entry.type,
                })),
              ]}
            />
          </article>
        ))}
      {step === 2 && (
        <>
          <h2 className="text-title-sm font-medium">{draft.title}</h2>
          <ul>
            {draft.changes.map((item) => (
              <li key={item.id}>
                {item.name} ·{" "}
                {item.type === "app"
                  ? t(($) => $.application_deployment.application)
                  : item.type === "sql"
                    ? "SQL"
                    : t(($) => $.application_deployment.configuration)}{" "}
                · {item.version}
              </li>
            ))}
          </ul>
          <ReleaseGates key={order.revision} wsId={wsId} order={order} />
          <ApplicationReleaseResults order={order} />
        </>
      )}
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
      {saved && (
        <p role="status" className="text-caption">
          {t(($) => $.application_deployment.saved)}
        </p>
      )}
      <div className="flex justify-between gap-2">
        <Button
          variant="outline"
          disabled={save.isPending}
          onClick={() => void persist()}
        >
          {t(($) => $.application_deployment.save_draft)}
        </Button>
        <div className="flex gap-2">
          {step > 0 && (
            <Button variant="outline" onClick={() => setStep(step - 1)}>
              {t(($) => $.application_deployment.previous)}
            </Button>
          )}
          {step < 2 && (
            <Button
              disabled={save.isPending || !draft.changes.length}
              onClick={() => void persist(step + 1)}
            >
              {t(($) => $.application_deployment.next)}
            </Button>
          )}
        </div>
      </div>
      {configuring && query.data && (
        <ApplicationDeploymentWizard
          projectId={order.project_id}
          applicationId={configuring}
          environment={order.env}
          initial={query.data}
          onClose={() => setConfiguring(undefined)}
        />
      )}
    </section>
  );
}

function ApplicationReleaseResults({ order }: { order: ReleaseOrder }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const { role } = useCurrentMember(wsId);
  const execute = useExecuteApplicationRelease(wsId, order.id);
  const [acknowledged, setAcknowledged] = useState(false);
  const canExecute = role === "owner" || role === "admin";
  const expired =
    order.status === "deploying" &&
    Boolean(order.lease_until) &&
    Date.parse(order.lease_until!) < Date.now();
  const retry =
    order.status === "failed" || order.status === "partial" || expired;
  const ready =
    Boolean(order.gates?.length) &&
    order.gates!.every((gate) => !gate.required || gate.passed);
  return (
    <section className="space-y-3">
      <h2 className="text-title-sm font-medium">
        {t(($) => $.application_deployment.execution)}
      </h2>
      {!order.manifest.delivery?.results.length && (
        <p className="text-muted-foreground">
          {t(($) => $.application_deployment.execution_pending)}
        </p>
      )}
      {order.manifest.delivery?.results.map((result, index) => (
        <div
          key={`${result.id}-${result.node_id}-${index}`}
          className="rounded-lg border p-3"
        >
          <p>
            {
              order.manifest.delivery?.changes.find(
                (item) => item.id === result.id,
              )?.name
            }{" "}
            ·{" "}
            {
              order.manifest.delivery?.snapshot?.resources.find(
                (resource) => resource.id === result.node_id,
              )?.name
            }{" "}
            ·{" "}
            {t(($) => $.application_deployment.result_status, {
              returnObjects: true,
            })[result.status as "pending"] ?? result.status}
          </p>
          <p className="break-all text-caption text-muted-foreground">
            {
              order.manifest.delivery?.changes.find(
                (item) => item.id === result.id,
              )?.resolved_version
            }
          </p>
          <p className="break-all text-caption">{result.detail}</p>
        </div>
      ))}
      {retry && (
        <label className="flex items-start gap-2">
          <input
            type="checkbox"
            checked={acknowledged}
            onChange={(e) => setAcknowledged(e.target.checked)}
          />
          {t(($) => $.application_deployment.acknowledge)}
        </label>
      )}
      {canExecute && (order.status === "draft" || retry) && (
        <Button
          disabled={execute.isPending || (retry ? !acknowledged : !ready)}
          onClick={() =>
            execute.mutate({ expected_revision: order.revision, acknowledged })
          }
        >
          {retry
            ? t(($) => $.application_deployment.retry_execution)
            : t(($) => $.application_deployment.execute)}
        </Button>
      )}
      {execute.isError && (
        <p role="alert" className="text-destructive">
          {execute.error.message}
        </p>
      )}
    </section>
  );
}

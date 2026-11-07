"use client";

import { useRef, useState } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { projectListOptions } from "@multica/core/projects";
import {
  releaseOrdersOptions,
  releaseOrderOptions,
  releaseCandidatesOptions,
  useCreateReleaseOrder,
} from "@multica/core/workbench/releases";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@multica/ui/components/ui/dialog";
import { ConfigurationSelect } from "../projects/components/configuration-select";
import { AppLink, useNavigation } from "../navigation";
import { useT } from "../i18n";
import { ReleaseGates } from "./release-gates";

export function ReleasesPage() {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const project = navigation.searchParams.get("project") ?? "";
  const env = navigation.searchParams.get("env") ?? "";
  const projects = useQuery(projectListOptions(wsId));
  const orders = useInfiniteQuery(releaseOrdersOptions(wsId, project, env));
  const setFilter = (name: string, value: string) => {
    const params = new URLSearchParams(navigation.searchParams);
    if (value) params.set(name, value);
    else params.delete(name);
    navigation.replace(`${paths.releases()}?${params}`);
  };
  const rows = orders.data?.pages.flatMap((page) => page.releases) ?? [];
  return (
    <div className="flex h-full min-h-0 flex-col">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b p-4">
        <h1 className="text-title font-semibold">
          {t(($) => $.releases.title)}
        </h1>
        <CreateReleaseDialog key={project} initialProject={project} />
      </header>
      <div className="min-h-0 flex-1 space-y-4 overflow-auto p-4">
        <div className="grid max-w-2xl gap-3 sm:grid-cols-2">
          <ConfigurationSelect
            label={t(($) => $.releases.project)}
            value={project}
            onChange={(value) => setFilter("project", value)}
            items={[
              { value: "", label: t(($) => $.releases.all) },
              ...(projects.data ?? []).map((entry) => ({
                value: entry.id,
                label: entry.title,
              })),
            ]}
          />
          <ConfigurationSelect
            label={t(($) => $.releases.environment)}
            value={env}
            onChange={(value) => setFilter("env", value)}
            items={[
              { value: "", label: t(($) => $.releases.all) },
              ...(["staging", "prod"] as const).map((value) => ({
                value,
                label: t(($) => $.releases[value]),
              })),
            ]}
          />
        </div>
        {orders.isPending && (
          <p role="status">{t(($) => $.releases.loading)}</p>
        )}
        {orders.isError && (
          <div role="alert" className="space-y-2">
            <p>{t(($) => $.releases.failed)}</p>
            <Button variant="outline" onClick={() => void orders.refetch()}>
              {t(($) => $.releases.retry)}
            </Button>
          </div>
        )}
        {orders.isSuccess && rows.length === 0 && (
          <p className="text-muted-foreground">{t(($) => $.releases.empty)}</p>
        )}
        <ul className="space-y-3">
          {rows.map((order) => (
            <li key={order.id}>
              <AppLink
                href={paths.releaseDetail(order.id)}
                className="block space-y-2 rounded-xl border bg-surface p-4 hover:bg-surface-hover"
              >
                <div className="flex flex-wrap justify-between gap-2">
                  <strong>{order.identifier}</strong>
                  <span>{t(($) => $.releases.status[order.status])}</span>
                </div>
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.releases[order.env])} ·{" "}
                  {t(($) => $.releases.task_count, {
                    count: order.manifest.tasks.length,
                  })}{" "}
                  ·{" "}
                  {t(($) => $.releases.repo_count, {
                    count: order.manifest.repositories.length,
                  })}
                </p>
                <p className="truncate text-caption">
                  {order.manifest.tasks.map((task) => task.title).join(" · ")}
                </p>
              </AppLink>
            </li>
          ))}
        </ul>
        {orders.hasNextPage && (
          <Button
            variant="outline"
            disabled={orders.isFetchingNextPage}
            onClick={() => void orders.fetchNextPage()}
          >
            {t(($) => $.releases.more)}
          </Button>
        )}
      </div>
    </div>
  );
}

function CreateReleaseDialog({ initialProject }: { initialProject: string }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const [open, setOpen] = useState(false);
  const [project, setProject] = useState(initialProject);
  const [env, setEnv] = useState<"staging" | "prod">("staging");
  const [excluded, setExcluded] = useState<string[]>([]);
  const [tags, setTags] = useState<Record<string, string>>({});
  const [error, setError] = useState<string>();
  const attempt = useRef<{ payload: string; id: string }>(undefined);
  const projects = useQuery(projectListOptions(wsId));
  const candidates = useInfiniteQuery({
    ...releaseCandidatesOptions(wsId, project, env),
    enabled: open && Boolean(project),
  });
  const create = useCreateReleaseOrder(wsId);
  const rows = candidates.data?.pages.flatMap((page) => page.candidates) ?? [];
  const selected = rows.filter((row) => !excluded.includes(row.id));
  const repositories = [
    ...new Map(
      selected
        .flatMap((row) => row.merges)
        .map((merge) => [merge.repo_id, merge]),
    ).values(),
  ];
  const resetSelection = () => {
    setExcluded([]);
    setTags({});
    setError(undefined);
  };
  const submit = async () => {
    const input = {
      project_id: project,
      env,
      tasks: selected.map((row) => ({
        id: row.id,
        expected_revision: row.revision,
      })),
      tags: Object.fromEntries(
        repositories
          .filter((repo) => tags[repo.repo_id]?.trim())
          .map((repo) => [repo.repo_id, tags[repo.repo_id]!.trim()]),
      ),
    };
    const payload = JSON.stringify(input);
    if (attempt.current?.payload !== payload)
      attempt.current = { payload, id: crypto.randomUUID() };
    setError(undefined);
    try {
      const order = await create.mutateAsync({
        ...input,
        request_id: attempt.current.id,
      });
      setOpen(false);
      navigation.push(paths.releaseDetail(order.id));
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t(($) => $.releases.failed),
      );
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
        {t(($) => $.releases.create)}
      </DialogTrigger>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $.releases.create)}</DialogTitle>
        </DialogHeader>
        <ConfigurationSelect
          label={t(($) => $.releases.project)}
          value={project}
          disabled={create.isPending}
          onChange={(value) => {
            setProject(value);
            resetSelection();
          }}
          items={[
            { value: "", label: t(($) => $.releases.choose_project) },
            ...(projects.data ?? []).map((entry) => ({
              value: entry.id,
              label: entry.title,
            })),
          ]}
        />
        <ConfigurationSelect
          label={t(($) => $.releases.environment)}
          value={env}
          disabled={create.isPending}
          onChange={(value) => {
            setEnv(value === "prod" ? "prod" : "staging");
            resetSelection();
          }}
          items={(["staging", "prod"] as const).map((value) => ({
            value,
            label: t(($) => $.releases[value]),
          }))}
        />
        {project && candidates.isPending && (
          <p role="status">{t(($) => $.releases.loading)}</p>
        )}
        {candidates.isError && (
          <div role="alert">
            <p>{t(($) => $.releases.failed)}</p>
            <Button variant="outline" onClick={() => void candidates.refetch()}>
              {t(($) => $.releases.retry)}
            </Button>
          </div>
        )}
        {candidates.isSuccess && rows.length === 0 && (
          <p className="text-caption text-muted-foreground">
            {t(($) => $.releases.no_candidates)}
          </p>
        )}
        <div className="space-y-2">
          {rows.map((row) => (
            <label key={row.id} className="flex items-start gap-2 text-caption">
              <input
                type="checkbox"
                disabled={create.isPending}
                checked={!excluded.includes(row.id)}
                onChange={(event) =>
                  setExcluded(
                    event.target.checked
                      ? excluded.filter((id) => id !== row.id)
                      : [...excluded, row.id],
                  )
                }
              />
              <span>
                {row.identifier} · {row.title}
              </span>
            </label>
          ))}
        </div>
        {candidates.hasNextPage && (
          <Button
            variant="outline"
            disabled={candidates.isFetchingNextPage || create.isPending}
            onClick={() => void candidates.fetchNextPage()}
          >
            {t(($) => $.releases.more)}
          </Button>
        )}
        {repositories.map((repo) => (
          <label key={repo.repo_id} className="block space-y-1 text-caption">
            <span className="block break-all">
              {t(($) => $.releases.version)} · {repo.url}
            </span>
            <Input
              disabled={create.isPending}
              maxLength={64}
              value={tags[repo.repo_id] ?? ""}
              placeholder={t(($) => $.releases.auto_version)}
              onChange={(event) =>
                setTags({ ...tags, [repo.repo_id]: event.target.value })
              }
            />
          </label>
        ))}
        {error && (
          <p role="alert" className="text-caption text-destructive">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            disabled={create.isPending}
            onClick={() => setOpen(false)}
          >
            {t(($) => $.releases.cancel)}
          </Button>
          <Button
            disabled={
              create.isPending ||
              candidates.isError ||
              selected.length === 0 ||
              selected.length > 100
            }
            aria-busy={create.isPending}
            onClick={() => void submit()}
          >
            {t(($) => $.releases.create)}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

export function ReleaseDetailPage({ releaseId }: { releaseId: string }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const order = useQuery(releaseOrderOptions(wsId, releaseId));
  return (
    <div className="h-full space-y-4 overflow-auto p-4">
      <AppLink href={paths.releases()} className="text-brand underline">
        {t(($) => $.releases.title)}
      </AppLink>
      {order.isPending && <p role="status">{t(($) => $.releases.loading)}</p>}
      {order.isError && (
        <div role="alert">
          <p>{t(($) => $.releases.failed)}</p>
          <Button variant="outline" onClick={() => void order.refetch()}>
            {t(($) => $.releases.retry)}
          </Button>
        </div>
      )}
      {order.data && (
        <>
          <header className="space-y-2">
            <h1 className="text-title font-semibold">
              {order.data.identifier}
            </h1>
            <p>
              {t(($) => $.releases[order.data.env])} ·{" "}
              {t(($) => $.releases.status[order.data.status])}
            </p>
          </header>
          <ReleaseGates
            key={`${order.data.id}-${order.data.revision}`}
            wsId={wsId}
            order={order.data}
          />
          <div className="grid gap-4 lg:grid-cols-2">
            <section className="space-y-3 rounded-xl border p-4">
              <h2 className="text-title-sm font-medium">
                {t(($) => $.releases.tasks)}
              </h2>
              <ul className="space-y-2">
                {order.data.manifest.tasks.map((task) => (
                  <li key={task.id}>
                    <AppLink
                      className="text-brand underline"
                      href={paths.issueDetail(task.id)}
                    >
                      {task.identifier} · {task.title}
                    </AppLink>
                  </li>
                ))}
              </ul>
            </section>
            <section className="space-y-3 rounded-xl border p-4">
              <h2 className="text-title-sm font-medium">
                {t(($) => $.releases.repositories)}
              </h2>
              {order.data.manifest.repositories.map((repo) => (
                <div key={repo.repo_id} className="space-y-2 text-caption">
                  <p className="font-medium">{repo.tag}</p>
                  {repo.merges.map((merge, index) => (
                    <div key={`${merge.pull_request_id}-${index}`}>
                      <a
                        className="block truncate text-brand underline"
                        href={merge.url}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        {merge.url}
                      </a>
                      <p
                        className="truncate text-muted-foreground"
                        title={merge.merge_commit}
                      >
                        {merge.base_branch} · <code>{merge.merge_commit}</code>
                      </p>
                    </div>
                  ))}
                </div>
              ))}
            </section>
          </div>
        </>
      )}
    </div>
  );
}

"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { projectResourcesOptions } from "@multica/core/projects";
import {
  repositoryProfileOptions,
  repositoryStackCatalogOptions,
  useSaveRepositoryProfile,
  type RepositoryProfileInput,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";
import { RepositoryProfileFields } from "./repository-profile-fields";

export function RepositoryProfilesSection({
  projectId,
}: {
  projectId: string;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const resources = useQuery(projectResourcesOptions(wsId, projectId));
  const [selected, setSelected] = useState<string>();
  const repository =
    resources.data?.find((resource) => resource.id === selected) ??
    resources.data?.[0];
  return (
    <section className="space-y-3 rounded-xl border p-3">
      <h3 className="text-label font-medium">
        {t(($) => $.repository_profile.title)}
      </h3>
      {resources.isPending && (
        <p role="status">{t(($) => $.repository_profile.loading)}</p>
      )}
      {resources.isError && (
        <Button
          size="sm"
          variant="outline"
          onClick={() => void resources.refetch()}
        >
          {t(($) => $.repository_profile.retry)}
        </Button>
      )}
      {!resources.isPending && !resources.isError && !repository && (
        <p className="text-caption">{t(($) => $.repository_profile.empty)}</p>
      )}
      {repository && (
        <>
          <label className="block text-caption">
            {t(($) => $.repository_profile.repository)}
            <select
              className="block w-full rounded-md border bg-surface p-2"
              value={repository.id}
              onChange={(event) => setSelected(event.target.value)}
            >
              {resources.data?.map((resource) => (
                <option key={resource.id} value={resource.id}>
                  {resource.label || resource.id}
                </option>
              ))}
            </select>
          </label>
          <RepositoryProfileEditor
            key={repository.id}
            projectId={projectId}
            repositoryId={repository.id}
          />
        </>
      )}
    </section>
  );
}

function RepositoryProfileEditor({
  projectId,
  repositoryId,
}: {
  projectId: string;
  repositoryId: string;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const profile = useQuery(
    repositoryProfileOptions(wsId, projectId, repositoryId),
  );
  const catalog = useQuery(repositoryStackCatalogOptions(wsId));
  const save = useSaveRepositoryProfile(wsId, projectId, repositoryId);
  const [draft, setDraft] = useState<RepositoryProfileInput>();
  const [error, setError] = useState<string>();
  const templateNames: Record<string, string> = {
    "java-backend": t(($) => $.repository_profile.java_backend),
    "java8-legacy": t(($) => $.repository_profile.java8_legacy),
    "python-service": t(($) => $.repository_profile.python_service),
    "ai-app": t(($) => $.repository_profile.ai_app),
    weapp: t(($) => $.repository_profile.weapp),
    "web-frontend": t(($) => $.repository_profile.web_frontend),
    custom: t(($) => $.repository_profile.custom),
  };
  if (profile.isPending || catalog.isPending)
    return <p role="status">{t(($) => $.repository_profile.loading)}</p>;
  if (profile.isError || catalog.isError)
    return (
      <Button
        size="sm"
        variant="outline"
        onClick={() => {
          void profile.refetch();
          void catalog.refetch();
        }}
      >
        {t(($) => $.repository_profile.retry)}
      </Button>
    );
  const saved = profile.data;
  const submit = async () => {
    if (!draft) return;
    setError(undefined);
    try {
      await save.mutateAsync(draft);
      setDraft(undefined);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.repository_profile.failed),
      );
    }
  };
  if (!draft)
    return (
      <div className="space-y-2 text-caption">
        <p>
          {saved.revision === 0
            ? t(($) => $.repository_profile.unconfigured)
            : `${templateNames[saved.stack.template] ?? saved.stack.template} · v${saved.revision}`}
        </p>
        <p>{saved.stack.tags.join(" · ")}</p>
        <p>
          {t(($) => $.repository_profile.delivery)} ·{" "}
          {saved.delivery.join(" · ")}
        </p>
        <p>
          {saved.stack.detected
            ? t(($) => $.repository_profile.detection_recorded)
            : t(($) => $.repository_profile.undetected)}
        </p>
        <p>
          {saved.stack.baseline
            ? t(($) => $.repository_profile.scan_recorded)
            : t(($) => $.repository_profile.unscanned)}
        </p>
        <details>
          <summary>{t(($) => $.repository_profile.commands)}</summary>
          <dl className="space-y-2 py-2">
            {Object.entries(saved.stack.derived).map(([key, value]) => (
              <div key={key}>
                <dt className="font-medium">{key}</dt>
                <dd className="whitespace-pre-wrap break-all font-mono">
                  {Array.isArray(value) ? value.join("\n") : value || "—"}
                </dd>
              </div>
            ))}
          </dl>
          {Object.keys(saved.stack.overrides).length > 0 && (
            <>
              <p>{t(($) => $.repository_profile.manual)}</p>
              <pre className="whitespace-pre-wrap break-all">
                {Object.entries(saved.stack.overrides)
                  .map(([key, value]) => `${key}\n${value.value}`)
                  .join("\n")}
              </pre>
            </>
          )}
        </details>
        {saved.stack.requires.length > 0 && (
          <p>
            {t(($) => $.repository_profile.preflight)} ·{" "}
            {saved.stack.requires.join(" · ")}
          </p>
        )}
        {["owner", "admin", "member"].includes(member.role ?? "") && (
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setError(undefined);
              setDraft({
                expected_revision: saved.revision,
                template: saved.stack.template,
                tags: saved.stack.tags,
                delivery: saved.delivery,
                overrides: Object.fromEntries(
                  Object.entries(saved.stack.overrides).map(([key, value]) => [
                    key,
                    value.value,
                  ]),
                ),
              });
            }}
          >
            {t(($) => $.repository_profile.edit)}
          </Button>
        )}
      </div>
    );
  return (
    <fieldset disabled={save.isPending} className="space-y-3 text-caption">
      <RepositoryProfileFields value={draft} onChange={setDraft} />
      <details>
        <summary>{t(($) => $.repository_profile.manual)}</summary>
        <div className="space-y-2 py-2">
          {["install", "build", "test", "lint", "scan"].map((command) => (
            <div key={command}>
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={command in draft.overrides}
                  onChange={(event) => {
                    const overrides = { ...draft.overrides };
                    if (event.target.checked) overrides[command] = "";
                    else delete overrides[command];
                    setDraft({ ...draft, overrides });
                  }}
                />
                {command}
              </label>
              {command in draft.overrides && (
                <Textarea
                  aria-label={command}
                  value={draft.overrides[command]}
                  onChange={(event) =>
                    setDraft({
                      ...draft,
                      overrides: {
                        ...draft.overrides,
                        [command]: event.target.value,
                      },
                    })
                  }
                />
              )}
            </div>
          ))}
        </div>
      </details>
      <div className="flex gap-2">
        <Button
          size="sm"
          disabled={!draft.delivery.length}
          aria-busy={save.isPending}
          onClick={() => void submit()}
        >
          {t(($) => $.repository_profile.save)}
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setDraft(undefined)}>
          {t(($) => $.repository_profile.cancel)}
        </Button>
      </div>
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
    </fieldset>
  );
}

"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectListOptions } from "@multica/core/projects";
import {
  credentialReferencesOptions,
  credentialsOptions,
  useDeleteWorkbenchCredential,
  useSaveWorkbenchCredential,
  type CredentialInput,
  type WorkbenchCredential,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ConfigurationSelect } from "../../projects/components/configuration-select";
import { useLocale, useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { SettingsSection } from "./settings-layout";
import { downloadBlob } from "../../editor/utils/mermaid-export";

export function WorkbenchCredentials() {
  const { t } = useT("projects");
  const locale = useLocale();
  const selectedCredential = useNavigation().searchParams.get("credential");
  const wsId = useWorkspaceId();
  const credentials = useQuery(credentialsOptions(wsId));
  const projects = useQuery(projectListOptions(wsId));
  const save = useSaveWorkbenchCredential(wsId);
  const remove = useDeleteWorkbenchCredential(wsId);
  const [draft, setDraft] = useState<{ id?: string; data: CredentialInput }>();
  const [deleting, setDeleting] = useState<WorkbenchCredential>();
  const [referenceId, setReferenceId] = useState("");
  const references = useQuery(credentialReferencesOptions(wsId, referenceId));
  const [error, setError] = useState<string>();
  const [exporting, setExporting] = useState(false);
  const exportAudit = async () => {
    setExporting(true);
    setError(undefined);
    try {
      downloadBlob(await api.exportWorkbenchAudit(), `workbench-audit-${new Date().toISOString().slice(0, 10)}.csv`);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t(($) => $.workbench.load_failed));
    } finally {
      setExporting(false);
    }
  };
  const usages = ["read-repo", "push-branch", "merge", "deploy"] as const;
  const change = (patch: Partial<CredentialInput>) =>
    setDraft(
      (current) =>
        current && { ...current, data: { ...current.data, ...patch } },
    );
  const start = (credential?: WorkbenchCredential) => {
    setError(undefined);
    setDraft(
      credential
        ? {
            id: credential.id,
            data: {
              ...credential,
              secret: "",
              expected_revision: credential.revision,
            },
          }
        : {
            data: {
              label: "",
              kind: "token",
              project_id: null,
              usage: ["deploy"],
              secret: "",
              expires_at: null,
            },
          },
    );
  };
  const submit = async () => {
    if (!draft) return;
    setError(undefined);
    try {
      await save.mutateAsync({ id: draft.id, data: { ...draft.data } });
      setDraft(undefined);
      save.reset();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.workbench.load_failed),
      );
      save.reset();
    }
  };
  const deleteCredential = async () => {
    if (!deleting) return;
    try {
      await remove.mutateAsync(deleting);
      setDeleting(undefined);
      setReferenceId("");
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.workbench.load_failed),
      );
    }
  };

  return (
    <SettingsSection title={t(($) => $.workbench.credentials)}>
      <div className="space-y-3">
        {credentials.isPending && <Skeleton className="h-16 w-full" />}
        {credentials.isError && (
          <p role="alert">
            {t(($) => $.workbench.load_failed)}{" "}
            <Button variant="ghost" onClick={() => void credentials.refetch()}>
              {t(($) => $.workbench.retry)}
            </Button>
          </p>
        )}
        {credentials.data && !credentials.data.encryption_configured && (
          <p className="text-caption text-warning">
            {t(($) => $.workbench.encryption_missing)}
          </p>
        )}
        <Button
          variant="outline"
          disabled={!credentials.data?.encryption_configured || !!draft}
          onClick={() => start()}
        >
          {t(($) => $.workbench.add)}
        </Button>
        <Button variant="outline" disabled={exporting} onClick={() => void exportAudit()}>
          {exporting ? t(($) => $.workbench.exporting_audit) : t(($) => $.workbench.export_audit)}
        </Button>
        {credentials.data?.credentials.map((credential) => (
          <div
            key={credential.id}
            className={`flex flex-wrap items-center gap-2 rounded-xl border bg-surface p-3 ${selectedCredential === credential.id ? "border-primary ring-1 ring-primary" : ""}`}
          >
            <span className="font-medium">{credential.label}</span>
            <span className="text-caption text-muted-foreground">
              ••••{credential.last_four}
            </span>
            {credential.expires_at && (
              <span className="text-caption text-muted-foreground">
                {t(($) => $.workbench.expires)} {new Date(credential.expires_at).toLocaleDateString(locale)}
              </span>
            )}
            <Button
              variant="ghost"
              size="sm"
              disabled={!credentials.data?.encryption_configured || !!draft}
              onClick={() => start(credential)}
            >
              {t(($) => $.workbench.rotate)}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setReferenceId(credential.id)}
            >
              {t(($) => $.workbench.references)}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDeleting(credential)}
            >
              {t(($) => $.workbench.remove)}
            </Button>
          </div>
        ))}
        {referenceId && (
          <div className="space-y-2 rounded-xl border p-3">
            <h4>{t(($) => $.workbench.references)}</h4>
            {references.isPending && <Skeleton className="h-8" />}
            {references.isError && (
              <p role="alert">{t(($) => $.workbench.load_failed)}</p>
            )}
            {references.data?.targets.length === 0 && (
              <p className="text-caption">{t(($) => $.workbench.not_set)}</p>
            )}
            {references.data?.targets.map((target) => (
              <p className="text-caption" key={target.id}>
                {
                  projects.data?.find(
                    (project) => project.id === target.project_id,
                  )?.title
                }{" "}
                · {target.label} · {target.env}
              </p>
            ))}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setReferenceId("")}
            >
              {t(($) => $.workbench.cancel)}
            </Button>
          </div>
        )}
        {deleting && (
          <div className="space-y-2 rounded-xl border p-3">
            <p>
              {t(($) => $.workbench.delete_confirm)} {deleting.label}
            </p>
            <Button
              variant="destructive"
              disabled={remove.isPending}
              onClick={() => void deleteCredential()}
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
        {draft && (
          <form
            className="space-y-3 rounded-xl border p-3"
            onSubmit={(event) => {
              event.preventDefault();
              void submit();
            }}
          >
            <fieldset className="space-y-3" disabled={save.isPending}>
              {!draft.id && (
                <>
                  <label className="block text-caption">
                    {t(($) => $.workbench.name)}
                    <Input
                      required
                      maxLength={120}
                      value={draft.data.label}
                      onChange={(event) =>
                        change({ label: event.target.value })
                      }
                    />
                  </label>
                  <ConfigurationSelect
                    label={t(($) => $.workbench.kind)}
                    value={draft.data.kind}
                    items={[
                      "token",
                      "ssh-key",
                      "ssh-password",
                      "cloud-ak",
                      "weapp-ci-key",
                      "registry-token",
                    ].map((kind) => ({ value: kind, label: kind }))}
                    onChange={(kind) => change({ kind })}
                  />
                  <ConfigurationSelect
                    label={t(($) => $.workbench.scope)}
                    value={draft.data.project_id ?? ""}
                    items={[
                      { value: "", label: t(($) => $.workbench.workspace) },
                      ...(projects.data ?? []).map((project) => ({
                        value: project.id,
                        label: project.title,
                      })),
                    ]}
                    onChange={(project_id) =>
                      change({ project_id: project_id || null })
                    }
                  />
                  <fieldset className="space-y-2">
                    <legend className="text-caption">
                      {t(($) => $.workbench.usage)}
                    </legend>
                    {usages.map((usage) => (
                      <label
                        key={usage}
                        className="flex items-center gap-2 text-caption"
                      >
                        <input
                          type="checkbox"
                          checked={draft.data.usage.includes(usage)}
                          onChange={(event) =>
                            change({
                              usage: event.target.checked
                                ? usage === "deploy"
                                  ? [usage]
                                  : [
                                      ...draft.data.usage.filter(
                                        (item) => item !== "deploy",
                                      ),
                                      usage,
                                    ]
                                : draft.data.usage.filter(
                                    (item) => item !== usage,
                                  ),
                            })
                          }
                        />
                        {t(($) => $.workbench[usage])}
                      </label>
                    ))}
                  </fieldset>
                </>
              )}
              <label className="block text-caption">
                {t(($) => $.workbench.secret)}
                <Input
                  type="password"
                  autoComplete="new-password"
                  required
                  maxLength={65536}
                  value={draft.data.secret}
                  onChange={(event) => change({ secret: event.target.value })}
                />
              </label>
              <label className="block text-caption">
                {t(($) => $.workbench.expires)}
                <Input
                  type="date"
                  value={draft.data.expires_at?.slice(0, 10) ?? ""}
                  onChange={(event) =>
                    change({
                      expires_at: event.target.value
                        ? `${event.target.value}T23:59:59Z`
                        : null,
                    })
                  }
                />
              </label>
              <div className="flex gap-2">
                <Button
                  type="submit"
                  disabled={save.isPending || draft.data.usage.length === 0}
                  aria-busy={save.isPending}
                >
                  {t(($) => $.workbench.save)}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={save.isPending}
                  onClick={() => setDraft(undefined)}
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
      </div>
    </SettingsSection>
  );
}

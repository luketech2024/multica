"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import {
  propertyListOptions,
  useCreateProperty,
  useSetIssueProperty,
  useUnsetIssueProperty,
} from "@multica/core/properties";
import {
  deployTargetsOptions,
  previewPropertyName,
  previewPath,
  previewURL,
  validPreviewURL,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";

export function TaskPreview({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const properties = useQuery(propertyListOptions(wsId));
  const targets = useQuery({
    ...deployTargetsOptions(wsId, issue.project_id ?? ""),
    enabled: !!issue.project_id,
  });
  const create = useCreateProperty();
  const save = useSetIssueProperty();
  const clear = useUnsetIssueProperty();
  const [draft, setDraft] = useState<string>();
  const [error, setError] = useState<string>();
  if (!issue.project_id) return null;
  const definition = properties.data?.find(
    (property) => property.name === previewPropertyName,
  );
  const value = definition && issue.properties?.[definition.id];
  const saved = typeof value === "string" ? value : "";
  const editable = ["owner", "admin", "member"].includes(member.role ?? "");
  const pending = create.isPending || save.isPending || clear.isPending;
  const configure = async () => {
    setError(undefined);
    try {
      await create.mutateAsync({
        name: previewPropertyName,
        type: "url",
        icon: "globe-2",
      });
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t(($) => $.preview.failed),
      );
    }
  };
  const submit = async () => {
    if (!definition || draft === undefined) return;
    setError(undefined);
    try {
      if (draft.trim()) {
        await save.mutateAsync({
          issueId: issue.id,
          propertyId: definition.id,
          value: draft.trim(),
        });
      } else {
        await clear.mutateAsync({
          issueId: issue.id,
          propertyId: definition.id,
        });
      }
      setDraft(undefined);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t(($) => $.preview.failed),
      );
    }
  };
  return (
    <section
      className="space-y-2 rounded-xl border p-3 text-caption"
      aria-label={t(($) => $.preview.title)}
    >
      <h3 className="text-label font-medium">{t(($) => $.preview.title)}</h3>
      {properties.isPending ? (
        <p role="status">{t(($) => $.preview.loading)}</p>
      ) : properties.isError ? (
        <Button
          size="sm"
          variant="outline"
          onClick={() => void properties.refetch()}
        >
          {t(($) => $.preview.retry)}
        </Button>
      ) : !definition ? (
        <>
          <p>{t(($) => $.preview.not_configured)}</p>
          {["owner", "admin"].includes(member.role ?? "") && (
            <Button
              size="sm"
              variant="outline"
              disabled={pending}
              onClick={() => void configure()}
            >
              {t(($) => $.preview.configure)}
            </Button>
          )}
        </>
      ) : definition.type !== "url" ? (
        <p role="alert">{t(($) => $.preview.wrong_type)}</p>
      ) : (
        <>
          {saved && validPreviewURL(saved) ? (
            <a
              href={saved}
              target="_blank"
              rel="noopener noreferrer"
              className="break-all underline"
            >
              {t(($) => $.preview.open)}
            </a>
          ) : (
            <p>{t(($) => $.preview.unpublished)}</p>
          )}
          {editable && (
            <fieldset disabled={pending} className="space-y-2">
              <label className="block">
                {t(($) => $.preview.address)}
                <Input
                  value={draft ?? saved}
                  onChange={(event) => setDraft(event.target.value)}
                />
              </label>
              <p className="break-all text-muted-foreground">
                {previewPath(issue.project_id, issue.identifier)}
              </p>
              {targets.isError && (
                <Button
                  size="xs"
                  variant="outline"
                  onClick={() => void targets.refetch()}
                >
                  {t(($) => $.preview.retry_targets)}
                </Button>
              )}
              {(targets.data?.targets ?? [])
                .filter((target) => target.env === "preview")
                .map((target) => {
                  const suggestion = previewURL(
                    target.location.domain ?? "",
                    issue.project_id!,
                    issue.identifier,
                  );
                  return suggestion ? (
                    <Button
                      key={target.id}
                      size="xs"
                      variant="outline"
                      onClick={() => setDraft(suggestion)}
                    >
                      {t(($) => $.preview.use_target, { name: target.label })}
                    </Button>
                  ) : null;
                })}
              {draft !== undefined && (
                <>
                  <p className="text-muted-foreground">
                    {t(($) => $.preview.record_hint)}
                  </p>
                  <Button
                    size="sm"
                    disabled={!!draft.trim() && !validPreviewURL(draft.trim())}
                    onClick={() => void submit()}
                  >
                    {t(($) => $.preview.save)}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setDraft(undefined)}
                  >
                    {t(($) => $.requirements.discard)}
                  </Button>
                </>
              )}
            </fieldset>
          )}
        </>
      )}
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
    </section>
  );
}

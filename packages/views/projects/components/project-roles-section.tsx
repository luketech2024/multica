"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import {
  projectRolesOptions,
  useSaveProjectRoles,
  type ProjectRoles,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ProjectRolesFields } from "./project-roles-fields";
import { useT } from "../../i18n";

export function ProjectRolesSection({ projectId }: { projectId: string }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const canEdit = ["owner", "admin", "member"].includes(member.role ?? "");
  const saved = useQuery(projectRolesOptions(wsId, projectId));
  const save = useSaveProjectRoles(wsId, projectId);
  const [draft, setDraft] = useState<ProjectRoles>();
  const [error, setError] = useState<string>();
  const [initializing, setInitializing] = useState(false);
  const current = draft ?? saved.data;
  const submit = async () => {
    if (!draft || initializing) return;
    setError(undefined);
    try {
      await save.mutateAsync(draft);
      setDraft(undefined);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.workbench.load_failed),
      );
    }
  };

  return (
    <section className="space-y-3" aria-label={t(($) => $.roles.title)}>
      <h3 className="text-label font-medium">{t(($) => $.roles.title)}</h3>
      {saved.isPending && <Skeleton className="h-24 w-full" />}
      {saved.isError && (
        <p role="alert">
          {t(($) => $.workbench.load_failed)}{" "}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void saved.refetch()}
          >
            {t(($) => $.workbench.retry)}
          </Button>
        </p>
      )}
      {current && (
        <fieldset disabled={!canEdit || save.isPending || initializing} className="space-y-2">
          <ProjectRolesFields
            config={current.config}
            onBusyChange={setInitializing}
            disabled={!canEdit || save.isPending}
            onChange={(config) => setDraft({ ...current, config })}
          />
          {draft && (
            <div className="flex gap-2">
              <Button
                disabled={save.isPending}
                aria-busy={save.isPending}
                onClick={() => void submit()}
              >
                {t(($) => $.workbench.save)}
              </Button>
              <Button
                variant="outline"
                disabled={save.isPending}
                onClick={() => {
                  setDraft(undefined);
                  setError(undefined);
                }}
              >
                {t(($) => $.workbench.cancel)}
              </Button>
            </div>
          )}
        </fieldset>
      )}
      {error && (
        <p role="alert" className="text-caption text-destructive">
          {error}
        </p>
      )}
    </section>
  );
}

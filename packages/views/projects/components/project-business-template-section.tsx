"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import {
  projectTemplateOptions,
  useSaveProjectTemplate,
  type ProjectTemplate,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { BusinessTemplateContent } from "./business-template-content";
import { BusinessTemplateEditor } from "./business-template-editor";
import { useT } from "../../i18n";

export function ProjectBusinessTemplateSection({
  projectId,
}: {
  projectId: string;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const canEdit = ["owner", "admin", "member"].includes(member.role ?? "");
  const save = useSaveProjectTemplate(wsId, projectId);
  const [draft, setDraft] = useState<ProjectTemplate>();
  const query = useQuery(projectTemplateOptions(wsId, projectId));
  if (query.isPending) return <Skeleton className="h-16 w-full" />;
  if (query.isError)
    return (
      <p role="alert" className="text-caption text-destructive">
        {t(($) => $.workbench.load_failed)}
        <Button variant="ghost" size="sm" onClick={() => void query.refetch()}>
          {t(($) => $.workbench.retry)}
        </Button>
      </p>
    );
  const template = query.data?.template;
  if (!template) return null;
  return (
    <section className="space-y-3" aria-label={t(($) => $.templates.title)}>
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-label font-medium">
          {t(($) => $.templates.title)}
        </h3>
        {canEdit && !draft && (
          <Button variant="ghost" size="sm" onClick={() => setDraft(template)}>
            {t(($) => $.workbench.edit)}
          </Button>
        )}
      </div>
      {draft ? (
        <BusinessTemplateEditor
          initial={draft.content}
          pending={save.isPending || !canEdit}
          onCancel={() => setDraft(undefined)}
          onSave={async (content) => {
            await save.mutateAsync({ ...draft, content });
            setDraft(undefined);
          }}
        />
      ) : (
        <details>
          <summary className="cursor-pointer text-caption">
            {template.content.name}
          </summary>
          <div className="mt-3">
            <BusinessTemplateContent template={template.content} />
          </div>
        </details>
      )}
    </section>
  );
}

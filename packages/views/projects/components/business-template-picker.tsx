"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  businessTemplatesOptions,
  type BusinessTemplateAsset,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { BusinessTemplateContent } from "./business-template-content";
import { useT } from "../../i18n";

export function BusinessTemplatePicker({
  value,
  onChange,
}: {
  value?: { id: string; revision: number };
  onChange: (template: BusinessTemplateAsset) => void;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const query = useQuery(businessTemplatesOptions(wsId));
  const selected = query.data?.templates.find(
    (template) => template.id === value?.id,
  );
  return (
    <section className="space-y-3" aria-label={t(($) => $.templates.title)}>
      <h3 className="text-label font-medium">{t(($) => $.templates.title)}</h3>
      {query.isPending && <Skeleton className="h-20 w-full" />}
      {query.isError && (
        <p role="alert" className="text-caption text-destructive">
          {t(($) => $.workbench.load_failed)}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void query.refetch()}
          >
            {t(($) => $.workbench.retry)}
          </Button>
        </p>
      )}
      <div className="grid grid-cols-2 gap-2">
        {query.data?.templates.map((template) => (
          <Button
            key={template.id}
            variant="outline"
            aria-pressed={value?.id === template.id}
            className="h-auto min-w-0 flex-col items-start whitespace-normal py-3 text-left aria-pressed:bg-brand/10 aria-pressed:border-brand"
            onClick={() => onChange(template)}
          >
            <span>{template.content.name}</span>
            <span className="text-micro text-muted-foreground">
              {t(($) => $.templates.counts, {
                acceptance: template.content.ac_skeleton.length,
                forbidden: template.content.forbidden.length,
                dod: template.content.dod.length,
                roles: 6 - template.disabled_roles.length,
              })}
            </span>
          </Button>
        ))}
      </div>
      {selected && (
        <details>
          <summary className="cursor-pointer text-caption">
            {t(($) => $.templates.details)}
          </summary>
          <div className="mt-3">
            <BusinessTemplateContent template={selected.content} />
          </div>
        </details>
      )}
    </section>
  );
}

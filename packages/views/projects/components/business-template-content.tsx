"use client";

import type { BusinessTemplate } from "@multica/core/workbench/configuration";
import { useT } from "../../i18n";

export function BusinessTemplateContent({
  template,
}: {
  template: BusinessTemplate;
}) {
  const { t } = useT("projects");
  const fields = [
    { label: t(($) => $.templates.background), values: template.background },
    { label: t(($) => $.templates.audience), values: template.audience },
    { label: t(($) => $.templates.scope_out), values: template.scope_out },
    { label: t(($) => $.templates.acceptance), values: template.ac_skeleton },
    {
      label: t(($) => $.templates.forbidden),
      values: template.forbidden.map((item) => `${item.rule} · ${item.check}`),
    },
    { label: t(($) => $.templates.dod), values: template.dod },
    {
      label: t(($) => $.templates.resources),
      values: template.resources.map((item) => `${item.ref} (${item.version})`),
    },
    {
      label: t(($) => $.templates.terms),
      values: Object.entries(template.terms).map(
        ([term, meaning]) => `${term} · ${meaning}`,
      ),
    },
  ];
  return (
    <dl className="grid gap-3 text-caption sm:grid-cols-2">
      {fields.map((field) => (
        <div key={field.label} className="min-w-0 space-y-1">
          <dt className="font-medium">{field.label}</dt>
          <dd className="text-muted-foreground break-words">
            {field.values.length ? (
              <ul className="space-y-1">
                {field.values.map((value, index) => (
                  <li key={index}>{value}</li>
                ))}
              </ul>
            ) : (
              t(($) => $.workbench.not_set)
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}

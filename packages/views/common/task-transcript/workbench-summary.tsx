"use client";

import { useState, type ComponentProps } from "react";
import { parseWorkbenchSummary } from "@multica/core/issues/workbench-summary";
import { Button } from "@multica/ui/components/ui/button";
import { ReadonlyContent } from "../../editor";
import { useT } from "../../i18n";
import { redactSecrets } from "./redact";

export function WorkbenchRunSummary({ content, attachments }: Pick<ComponentProps<typeof ReadonlyContent>, "content" | "attachments">) {
  const { t } = useT("issues");
  const [expanded, setExpanded] = useState(false);
  const safeContent = redactSecrets(content);
  const summary = parseWorkbenchSummary(safeContent);
  if (!summary) return <ReadonlyContent content={safeContent} attachments={attachments} />;
  const sections = [
    [t(($) => $.run_summary.conclusion), summary.conclusion],
    [t(($) => $.run_summary.changes), summary.changes.join("\n")],
    [t(($) => $.run_summary.validation), summary.validation.join("\n")],
    [t(($) => $.run_summary.risks), summary.risks.join("\n")],
  ];
  return <section className="space-y-2 rounded-xl border bg-surface p-3" aria-label={t(($) => $.run_summary.title)}>
    <dl className="space-y-1 text-caption">
      {sections.map(([label, value]) => <div key={label} className="flex min-w-0 gap-2">
        <dt className="shrink-0 font-medium">{label}</dt>
        <dd className="line-clamp-1 min-w-0 whitespace-pre-wrap break-words" title={value || undefined}>{value || t(($) => $.run_summary.missing)}</dd>
      </div>)}
    </dl>
    <Button size="xs" variant="ghost" aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>{expanded ? t(($) => $.run_summary.collapse) : t(($) => $.run_summary.expand)}</Button>
    {expanded && <ReadonlyContent content={safeContent} attachments={attachments} />}
  </section>;
}

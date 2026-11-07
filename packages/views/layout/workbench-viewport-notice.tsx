"use client";

import { useT } from "../i18n";

export function WorkbenchViewportNotice() {
  const { t } = useT("layout");
  return <div role="status" className="hidden shrink-0 border-b border-surface-border bg-surface px-6 py-1 text-caption text-muted-foreground max-[1023px]:block">{t(($) => $.workbench_nav.narrow_viewport)}</div>;
}

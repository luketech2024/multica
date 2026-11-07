"use client";

import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";
import { createShortcutChord } from "@multica/core/shortcuts";
import { ActionShortcutKeycaps, ShortcutKeycaps } from "../common/shortcut-keycaps";
import { useT } from "../i18n";

export function WorkbenchShortcutHelp({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useT("issues");
  const rows = [
    { key: <ActionShortcutKeycaps action="openSearch" />, label: t(($) => $.workbench_keys.search) },
    { key: <span className="flex gap-1"><ShortcutKeycaps shortcut={createShortcutChord("J")} /><ShortcutKeycaps shortcut={createShortcutChord("K")} /></span>, label: t(($) => $.workbench_keys.move) },
    { key: <ShortcutKeycaps shortcut={createShortcutChord("Enter")} />, label: t(($) => $.workbench_keys.open) },
    { key: <ShortcutKeycaps shortcut={createShortcutChord("A")} />, label: t(($) => $.workbench_keys.review) },
    { key: <ActionShortcutKeycaps action="createIssue" />, label: t(($) => $.workbench_keys.create) },
    { key: <ShortcutKeycaps shortcut={createShortcutChord("Escape")} />, label: t(($) => $.workbench_keys.escape) },
    { key: <ActionShortcutKeycaps action="showShortcutHelp" />, label: t(($) => $.workbench_keys.help) },
  ];
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={undefined}>
        <DialogHeader><DialogTitle>{t(($) => $.workbench_keys.title)}</DialogTitle></DialogHeader>
        <dl className="space-y-3">
          {rows.map((row) => (
            <div key={row.label} className="flex items-center justify-between gap-4">
              <dt className="text-body">{row.label}</dt>
              <dd className="shrink-0">{row.key}</dd>
            </div>
          ))}
        </dl>
      </DialogContent>
    </Dialog>
  );
}

"use client";

import { useMemo, useState, type ReactNode } from "react";
import type { TimelineEntry } from "@multica/core/types";
import { isDeletedComment } from "@multica/core/issues/comment-deletion";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { Dialog, DialogTrigger, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@multica/ui/components/ui/dialog";
import { useLocale, useT } from "../../i18n";
import { descriptionPreview } from "./description-preview";

export function isReplyDiscussion(entry: TimelineEntry) {
  return entry.type === "comment" && !isDeletedComment(entry)
    && (!entry.comment_type || entry.comment_type === "comment")
    && entry.actor_type !== "system";
}

export function DiscussionReplyPicker({ entries, loading, failed, onRetry, onSelect, label, primary = false, onShowActivity }: {
  entries: TimelineEntry[];
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  onSelect: (entry: TimelineEntry) => void;
  label: ReactNode;
  primary?: boolean;
  onShowActivity: () => void;
}) {
  const { t } = useT("issues");
  const locale = useLocale();
  const { getActorName } = useActorName();
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<TimelineEntry | null>(null);
  const [visibleCount, setVisibleCount] = useState(20);
  // The timeline API returns the full history. Window the chooser locally.
  const choices = useMemo(() => entries.filter(isReplyDiscussion).toSorted((a, b) =>
    Date.parse(b.created_at) - Date.parse(a.created_at)), [entries]);
  return <Dialog open={open} onOpenChange={(value) => {
    setOpen(value);
    if (value) { setSelected(null); setVisibleCount(20); }
  }} onOpenChangeComplete={(value) => {
    if (!value && selected) { onSelect(selected); setSelected(null); }
  }}>
    <DialogTrigger render={<Button size={primary ? "default" : "sm"} variant={primary ? "default" : "ghost"} />}>
      {label}
    </DialogTrigger>
    <DialogContent className="sm:max-w-xl max-h-[85dvh] overflow-y-auto" finalFocus={selected ? false : undefined}>
      <DialogHeader>
        <DialogTitle>{t(($) => $.discussion_picker.title)}</DialogTitle>
        <DialogDescription>{t(($) => $.discussion_picker.description)}</DialogDescription>
      </DialogHeader>
      {loading ? <p role="status">{t(($) => $.discussion_picker.loading)}</p>
        : failed ? <div role="alert" className="space-y-3">
          <p>{t(($) => $.discussion_picker.failed)}</p>
          <Button variant="outline" onClick={onRetry}>{t(($) => $.release_readiness.retry)}</Button>
        </div>
        : choices.length === 0 ? <div className="space-y-3">
          <p>{t(($) => $.discussion_picker.empty)}</p>
          <Button variant="outline" onClick={() => { setOpen(false); onShowActivity(); }}>{t(($) => $.next_step.actions.task)}</Button>
        </div>
        : <div className="space-y-3">
          {choices.slice(0, visibleCount).map((entry) => <article key={entry.id} className="rounded-lg border p-4 space-y-3">
            <div className="flex flex-wrap justify-between gap-2 text-caption text-muted-foreground">
              <span>{entry.actor_name || getActorName(entry.actor_type, entry.actor_id)}</span>
              <time dateTime={entry.created_at}>{new Date(entry.created_at).toLocaleString(locale)}</time>
            </div>
            <p className="line-clamp-4 whitespace-pre-wrap break-words text-body">{descriptionPreview(entry.content ?? "") || t(($) => $.discussion_picker.attachment)}</p>
            {entry.parent_id && <p className="text-caption text-muted-foreground">{t(($) => $.discussion_picker.in_thread)}</p>}
            <Button variant="outline" onClick={() => { setSelected(entry); setOpen(false); }}>{t(($) => $.discussion_picker.reply)}</Button>
          </article>)}
          {choices.length > visibleCount && <Button variant="outline" onClick={() => setVisibleCount((count) => count + 20)}>{t(($) => $.discussion_picker.more)}</Button>}
        </div>}
    </DialogContent>
  </Dialog>;
}

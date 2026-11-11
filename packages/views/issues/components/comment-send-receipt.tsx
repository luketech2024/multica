"use client";

import type { Comment } from "@multica/core/types";
import { mentionLabelsByTarget, parseCommentTriggerOutcomes } from "@multica/core/issues/comment-trigger-outcomes";
import { useActorName } from "@multica/core/workspace/hooks";
import { blockedShortReasonLabel } from "../blocked-trigger-copy";
import { useT } from "../../i18n";

export function CommentSendReceipt({ comment }: { comment: Comment | null }) {
  const { t } = useT("issues");
  const { getActorName } = useActorName();
  if (!comment) return null;
  const labels = mentionLabelsByTarget(comment.content);
  const outcomes = parseCommentTriggerOutcomes(comment.trigger_outcomes);
  return <div role="status" className="mb-2 max-h-24 overflow-y-auto text-caption text-muted-foreground">
    <p>{t(($) => $.composer_ui.posted)}</p>
    {outcomes.map((outcome, index) => {
      const name = labels.get(`${outcome.target_type}:${outcome.target_id}`)
        ?? (outcome.target_type === "agent" ? getActorName("agent", outcome.target_id) : t(($) => $.composer_ui.recipient));
      const queued = ["queued", "coalesced", "deferred"].includes(outcome.status);
      const handled = queued || outcome.status === "steered";
      return <p key={`${outcome.target_id}-${index}`} className={handled ? undefined : "text-warning"}>
        {name} · {queued ? t(($) => $.composer_ui.queued) : outcome.status === "steered"
          ? t(($) => $.composer_ui.received) : blockedShortReasonLabel(outcome.reason_code, t)}
      </p>;
    })}
  </div>;
}

import type { BuiltInIssueStatus, IssueStatus } from "@multica/core/types";
import { isBuiltInIssueStatus } from "@multica/core/issue-statuses";
import { Badge } from "@multica/ui/components/ui/badge";
import { StatusIcon } from "./status-icon";
import { useWorkspaceId } from "@multica/core/hooks";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import { useStatusLabel } from "../utils/status-label";

const statusVariants = {
  backlog: "neutral",
  todo: "info",
  in_progress: "brand",
  in_review: "warning",
  blocked: "destructive",
  paused: "neutral",
  done: "success",
  cancelled: "neutral",
} as const satisfies Record<BuiltInIssueStatus, string>;

export function StatusHeading({
  status,
  count,
}: {
  status: IssueStatus;
  count: number;
}) {
  const wsId = useWorkspaceId();
  const labelOf = useStatusLabel(wsId);
  const { categoryOf, colorOf, iconOf } = useIssueStatuses(wsId);
  return (
    <div className="flex items-center gap-2">
      <Badge variant={isBuiltInIssueStatus(status) ? statusVariants[status] : "neutral"} className={status === "cancelled" ? "line-through" : undefined}>
        <StatusIcon category={categoryOf(status)} color={colorOf(status)} icon={iconOf(status)} status={status} className="h-3 w-3" />
        {labelOf(status)}
      </Badge>
      <span className="text-caption text-muted-foreground">{count}</span>
    </div>
  );
}

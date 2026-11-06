"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useCurrentMember } from "@multica/core/permissions";
import { securityChecksOptions } from "@multica/core/workbench/security";
import { Shield } from "@multica/ui/icons";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { PanelCount, PanelLink, isPathWithin } from "../layout/panel-nav";

export function SecurityNav() {
  const wsId = useWorkspaceId();
  const { role } = useCurrentMember(wsId);
  const enabled = role === "owner" || role === "admin" || role === "member";
  const { data } = useInfiniteQuery({ ...securityChecksOptions(wsId), enabled });
  const { t } = useT("layout");
  const paths = useWorkspacePaths();
  const { pathname } = useNavigation();
  const count = data?.pages[0]?.counts.filter((entry) => entry.effective_status !== "fixed" && entry.severity !== "low").reduce((sum, entry) => sum + entry.total, 0) ?? 0;
  if (!enabled) return null;
  return <PanelLink href={paths.security()} icon={<Shield />} label={t(($) => $.nav.security)}
    active={isPathWithin(pathname, paths.security())} trailing={count > 0 ? <PanelCount value={count} /> : undefined} />;
}

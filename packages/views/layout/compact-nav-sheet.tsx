"use client";

import { useEffect } from "react";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@multica/ui/components/ui/sheet";
import { useSidebar } from "@multica/ui/components/ui/sidebar";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { AppRail } from "./app-rail";

/**
 * Below `md` the rail and the current space's panel move into one sheet,
 * opened by the page header's nav trigger.
 */
export function CompactNavSheet({ globalActions = true }: { globalActions?: boolean }) {
  const { t } = useT("layout");
  const { isCompact, openMobile, setOpenMobile } = useSidebar();
  const { pathname, searchParams } = useNavigation();
  const search = searchParams.toString();

  // The sheet covers the page, so leaving through it has to dismiss it or
  // the destination renders underneath and the tap reads as nothing
  // happened. Keying on `pathname` covers every route out at once: rail
  // links, panel rows, pins, and the workspace switcher.
  useEffect(() => {
    setOpenMobile(false);
  }, [pathname, search, setOpenMobile]);

  if (!isCompact) return null;

  return (
    <Sheet open={openMobile} onOpenChange={setOpenMobile}>
      <SheetContent
        side="left"
        showCloseButton={false}
        data-slot="compact-nav-sheet"
        className="flex-row gap-0 overflow-hidden bg-app-shell p-0 data-[side=left]:w-auto data-[side=left]:max-w-[calc(100vw-3rem)]"
      >
        <SheetHeader className="sr-only">
          <SheetTitle>{t(($) => $.rail.label)}</SheetTitle>
          <SheetDescription>{t(($) => $.rail.sheet_description)}</SheetDescription>
        </SheetHeader>
        <AppRail globalActions={globalActions} />
      </SheetContent>
    </Sheet>
  );
}

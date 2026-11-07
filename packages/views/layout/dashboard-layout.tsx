"use client";

import type { ReactNode } from "react";
import { SidebarProvider } from "@multica/ui/components/ui/sidebar";
import { ModalRegistry } from "../modals/registry";
import { SourceBackfillModal } from "../onboarding";
import { AppRail } from "./app-rail";
import { AppShellProvider } from "./app-shell-context";
import { CompactNavSheet } from "./compact-nav-sheet";
import { DashboardGuard } from "./dashboard-guard";
import { NavigationProgress } from "./navigation-progress";
import { ShellRouteDataPrefetch } from "./route-data-prefetch";
import { WorkspacePresencePrefetch } from "./workspace-presence-prefetch";
import { GlobalShortcuts } from "./global-shortcuts";
import { WorkbenchViewportNotice } from "./workbench-viewport-notice";

interface DashboardLayoutProps {
  children: ReactNode;
  /** Rendered inside the content column (e.g. ChatWindow, ChatFab — absolute-positioned overlays) */
  extra?: ReactNode;
  /** Loading indicator */
  loadingIndicator?: ReactNode;
}

/** The shared page canvas beside the navigation rail. */
export function AppShellCard({ children }: { children: ReactNode }) {
  return (
    <main className="relative flex min-w-0 flex-1 flex-row overflow-hidden bg-page-canvas">
      <div className="relative flex min-w-0 flex-1 flex-col overflow-hidden"><WorkbenchViewportNotice />{children}</div>
    </main>
  );
}

export function DashboardLayout({
  children,
  extra,
  loadingIndicator,
}: DashboardLayoutProps) {
  return (
    <DashboardGuard
      loadingFallback={
        <div className="flex h-svh items-center justify-center">
          {loadingIndicator}
        </div>
      }
    >
      <SidebarProvider className="h-svh bg-app-shell [--sidebar-wrapper-fill:var(--app-shell)]">
        <AppShellProvider>
          <ShellRouteDataPrefetch>
            <GlobalShortcuts />
            <WorkspacePresencePrefetch />
            <AppRail className="hidden md:flex" />
            <CompactNavSheet />
            <AppShellCard>
              <NavigationProgress />
              {children}
              <ModalRegistry />
              <SourceBackfillModal />
              {extra}
            </AppShellCard>
          </ShellRouteDataPrefetch>
        </AppShellProvider>
      </SidebarProvider>
    </DashboardGuard>
  );
}

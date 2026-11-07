"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { Issue } from "@multica/core/types";
import { getShortcut, isEditableShortcutTarget, isPortalLayerShortcutTarget, shortcutMatchesEvent, SHORTCUT_ACTIONS } from "@multica/core/shortcuts";
import { isImeComposing } from "@multica/core/utils";
import { useModalStore } from "@multica/core/modals";
import { useIssuePeekOpen } from "./peek-context";

type RevealTask = (id: string) => boolean;
interface KeyboardContextValue {
  focusedId: string | null;
  setFocus: (id: string | null) => void;
  setOrder: (ids: string[]) => void;
  registerNode: (id: string, node: HTMLAnchorElement | null) => void;
  registerReveal: (key: symbol, reveal: RevealTask) => () => void;
}
const KeyboardContext = createContext<KeyboardContextValue | null>(null);

export function WorkbenchKeyboardProvider({ enabled, issues, children }: {
  enabled: boolean;
  issues: Issue[];
  children: ReactNode;
}) {
  const root = useRef<HTMLDivElement>(null);
  const nodes = useRef(new Map<string, HTMLAnchorElement>());
  const revealers = useRef(new Map<symbol, RevealTask>());
  const order = useRef<string[]>([]);
  const focused = useRef<string | null>(null);
  const [focusedId, setFocusedId] = useState<string | null>(null);
  const peekOpen = useIssuePeekOpen();
  const setFocus = useCallback((id: string | null) => {
    focused.current = id;
    setFocusedId(id);
  }, []);
  const setOrder = useCallback((ids: string[]) => {
    order.current = ids;
    if (focused.current && !ids.includes(focused.current)) setFocus(null);
  }, [setFocus]);
  const registerNode = useCallback((id: string, node: HTMLAnchorElement | null) => {
    if (!node) { nodes.current.delete(id); return; }
    nodes.current.set(id, node);
    if (focused.current === id) node.focus({ preventScroll: true });
  }, []);
  const registerReveal = useCallback((key: symbol, reveal: RevealTask) => {
    revealers.current.set(key, reveal);
    return () => { revealers.current.delete(key); };
  }, []);

  useEffect(() => {
    if (!enabled) return;
    const handleKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.repeat || isImeComposing(event) || peekOpen) return;
      if (event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return;
      if (!root.current?.getClientRects().length || root.current.closest("[inert], [hidden]")) return;
      if (isPortalLayerShortcutTarget(event.target) || useModalStore.getState().modal) return;
      if (event.key === "Escape") {
        if (!focused.current) return;
        event.preventDefault();
        nodes.current.get(focused.current)?.blur();
        setFocus(null);
        return;
      }
      if (isEditableShortcutTarget(event.target)) return;
      // User-configured shortcuts take precedence over the task reading keys.
      if (SHORTCUT_ACTIONS.some((action) => shortcutMatchesEvent(getShortcut(action.id), event))) return;
      const key = event.key.toLowerCase();
      if (key === "j" || key === "k") {
        const ids = order.current;
        if (!ids.length || !nodes.current.size) return;
        event.preventDefault();
        const current = focused.current ? ids.indexOf(focused.current) : -1;
        const index = current < 0 ? (key === "j" ? 0 : ids.length - 1)
          : Math.max(0, Math.min(ids.length - 1, current + (key === "j" ? 1 : -1)));
        const id = ids[index]!;
        setFocus(id);
        const node = nodes.current.get(id);
        if (node) {
          node.scrollIntoView({ block: "nearest", inline: "nearest" });
          node.focus({ preventScroll: true });
        } else {
          for (const reveal of revealers.current.values()) {
            if (reveal(id)) break;
          }
        }
        return;
      }
      if ((key === "enter" || key === "a") && focused.current) {
        const node = nodes.current.get(focused.current);
        if (event.target !== node) return;
        if (key === "a" && issues.find((issue) => issue.id === focused.current)?.status !== "in_review") return;
        event.preventDefault();
        node?.click();
      }
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [enabled, issues, peekOpen, setFocus]);

  const value = useMemo(() => ({ focusedId, setFocus, setOrder, registerNode, registerReveal }), [focusedId, setFocus, setOrder, registerNode, registerReveal]);
  if (!enabled) return children;
  return (
    <KeyboardContext.Provider value={value}>
      <div ref={root} className="flex min-h-0 flex-1 flex-col">{children}</div>
    </KeyboardContext.Provider>
  );
}

export function useWorkbenchKeyboardOrder(ids: string[]) {
  const setOrder = useContext(KeyboardContext)?.setOrder;
  useEffect(() => {
    setOrder?.(ids);
  }, [ids, setOrder]);
}

export function useWorkbenchKeyboardReveal(reveal: RevealTask) {
  const register = useContext(KeyboardContext)?.registerReveal;
  const key = useRef(Symbol("task-scroller"));
  useEffect(() => register?.(key.current, reveal), [register, reveal]);
}

export function useWorkbenchKeyboardTarget(id: string) {
  const context = useContext(KeyboardContext);
  const register = context?.registerNode;
  const ref = useCallback((node: HTMLAnchorElement | null) => register?.(id, node), [id, register]);
  return {
    ref,
    onFocus: () => context?.setFocus(id),
    "data-keyboard-selected": context?.focusedId === id ? "" : undefined,
  };
}

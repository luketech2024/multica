"use client";

import { use } from "react";
import { IssuePipelinePage } from "@multica/views/issues/components";
import { ErrorBoundary } from "@multica/ui/components/common/error-boundary";

export default function PipelinePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <ErrorBoundary resetKeys={[id]}><IssuePipelinePage routeId={id} /></ErrorBoundary>;
}

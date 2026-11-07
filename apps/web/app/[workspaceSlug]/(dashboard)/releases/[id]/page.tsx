"use client";

import { use } from "react";
import { ReleaseDetailPage } from "@multica/views/releases";

export default function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <ReleaseDetailPage releaseId={id} />;
}

import { useParams } from "react-router-dom";
import { ReleaseDetailPage as ReleaseDetailView } from "@multica/views/releases";

export function ReleaseDetailPage() {
  const { id } = useParams<{ id: string }>();
  return id ? <ReleaseDetailView releaseId={id} /> : null;
}

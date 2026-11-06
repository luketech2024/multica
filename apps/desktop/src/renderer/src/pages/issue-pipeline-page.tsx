import { useParams } from "react-router-dom";
import { IssuePipelinePage as PipelineView } from "@multica/views/issues/components";

export function IssuePipelinePage() {
  const { id } = useParams<{ id: string }>();
  return id ? <PipelineView routeId={id} /> : null;
}

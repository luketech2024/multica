-- Comment mutations and confirmation share the owning issue row lock.
CREATE FUNCTION workbench_discussion_changed(criteria jsonb) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
 SELECT CASE WHEN criteria->>'v' = '1' AND criteria#>>'{gate,spec}' IS NULL
 THEN criteria || jsonb_build_object(
   'phase', COALESCE(criteria->>'phase', CASE WHEN (CASE WHEN jsonb_typeof(criteria->'items') = 'array' THEN jsonb_array_length(criteria->'items') ELSE 0 END) > 0 THEN 'draft' ELSE 'clarifying' END),
   'discussion_revision', COALESCE((criteria->>'discussion_revision')::bigint, 0) + 1)
 ELSE criteria END
$$;

CREATE FUNCTION track_requirements_comment_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE original comment;
BEGIN
 original := OLD;
 IF original.author_type <> 'member' OR original.deleted_at IS NOT NULL THEN RETURN NULL; END IF;
 IF TG_OP = 'UPDATE' AND NEW.content IS NOT DISTINCT FROM OLD.content AND NEW.deleted_at IS NOT DISTINCT FROM OLD.deleted_at AND (NEW.revision = OLD.revision OR NEW.resolved_at IS DISTINCT FROM OLD.resolved_at) THEN RETURN NULL; END IF;
 IF original.content ~* '^\s*/note(\s|$)' AND (TG_OP = 'DELETE' OR NEW.content ~* '^\s*/note(\s|$)') THEN RETURN NULL; END IF;
 UPDATE issue SET acceptance_criteria = workbench_discussion_changed(acceptance_criteria), revision = revision + 1
 WHERE id = original.issue_id AND workspace_id = original.workspace_id
   AND parent_issue_id IS NULL AND project_id IS NOT NULL
   AND acceptance_criteria->>'v' = '1' AND acceptance_criteria#>>'{gate,spec}' IS NULL;
 RETURN NULL;
END
$$;
CREATE TRIGGER requirements_comment_change AFTER UPDATE OR DELETE ON comment
FOR EACH ROW EXECUTE FUNCTION track_requirements_comment_change();

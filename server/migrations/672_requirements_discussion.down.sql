DROP TRIGGER IF EXISTS requirements_comment_change ON comment;
DROP FUNCTION IF EXISTS track_requirements_comment_change();
DROP FUNCTION IF EXISTS workbench_discussion_changed(jsonb);

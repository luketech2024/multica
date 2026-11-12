CREATE OR REPLACE FUNCTION guard_onespec_execution()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE target issue; flow jsonb; phase text; role_name text; plan jsonb; approval jsonb; execution_snapshot jsonb;
BEGIN
 IF NEW.issue_id IS NULL OR NEW.status NOT IN ('queued','deferred','dispatched','running','waiting_local_directory') THEN RETURN NEW; END IF;
 IF TG_OP='UPDATE' AND NEW.status IS NOT DISTINCT FROM OLD.status AND NEW.agent_id IS NOT DISTINCT FROM OLD.agent_id AND NEW.runtime_id IS NOT DISTINCT FROM OLD.runtime_id THEN RETURN NEW; END IF;
 SELECT * INTO target FROM issue WHERE id=NEW.issue_id;
 SELECT state INTO flow FROM onespec_flow WHERE workspace_id=target.workspace_id AND issue_id=COALESCE(target.parent_issue_id,target.id) FOR SHARE;
 IF flow IS NULL THEN RETURN NEW; END IF;
 phase:=flow->>'phase';
 IF flow->>'runtime_id' IS DISTINCT FROM NEW.runtime_id::text THEN
  RAISE EXCEPTION 'Use the runtime captured by this OneSpec task' USING ERRCODE='23514',CONSTRAINT='workbench_execution_gate';
 END IF;
 IF target.parent_issue_id IS NOT NULL THEN
  SELECT value INTO plan FROM jsonb_array_elements(flow->'artifacts') WITH ORDINALITY a(value,n) WHERE value->>'stage'='plan' ORDER BY n DESC LIMIT 1;
  SELECT value INTO approval FROM jsonb_array_elements(flow->'decisions') WITH ORDINALITY d(value,n) WHERE value->>'stage'='plan' AND value->>'version'=plan->>'version' ORDER BY n DESC LIMIT 1;
  execution_snapshot:=jsonb_build_object('issue_id',target.parent_issue_id,'plan_version',plan->>'version');
  IF phase NOT IN ('implementation','testing') OR approval->>'approved' IS DISTINCT FROM 'true' OR plan->>'requirement_version' IS DISTINCT FROM target.properties #>> '{workbench_handoff,requirement_version}' THEN
   RAISE EXCEPTION 'Confirm and apply the current OneSpec plan before execution' USING ERRCODE='23514',CONSTRAINT='workbench_execution_gate';
  END IF;
 ELSE
  execution_snapshot:=jsonb_build_object('issue_id',target.id,'phase',phase,'generation',COALESCE(flow->>'generation','0'));
  IF flow->>'status' <> 'preparing' OR phase IN ('acceptance','complete') THEN
   RAISE EXCEPTION 'OneSpec is awaiting a human decision or stage advance' USING ERRCODE='23514',CONSTRAINT='workbench_execution_gate';
  END IF;
  role_name:=CASE phase WHEN 'requirements' THEN 'product' WHEN 'testing' THEN 'test' WHEN 'release_preparation' THEN 'ops' WHEN 'deployment' THEN 'ops' WHEN 'documentation' THEN 'docs' WHEN 'branch' THEN 'git' WHEN 'merge' THEN 'git' WHEN 'merging' THEN 'git' ELSE 'dev' END;
  IF flow #>> ARRAY['team','config','roles',role_name,'agent_id'] IS DISTINCT FROM NEW.agent_id::text THEN
   RAISE EXCEPTION 'Assign the agent responsible for the current OneSpec stage' USING ERRCODE='23514',CONSTRAINT='workbench_execution_gate';
  END IF;
 END IF;
 IF NEW.context->'onespec_execution' IS NOT NULL AND NEW.context->'onespec_execution' IS DISTINCT FROM execution_snapshot THEN
  RAISE EXCEPTION 'This execution belongs to an obsolete OneSpec stage or plan' USING ERRCODE='23514',CONSTRAINT='workbench_execution_gate';
 END IF;
 NEW.context:=jsonb_set(COALESCE(NULLIF(NEW.context,'null'::jsonb),'{}'::jsonb),'{onespec_execution}',execution_snapshot);
 RETURN NEW;
END $$;

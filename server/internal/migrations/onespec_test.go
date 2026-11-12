package migrations

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOneSpecMigrationsAndExecutionSnapshot(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	schema := pgx.Identifier{"onespec_test_" + uuid.NewString()}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err = conn.Exec(ctx, "SET search_path TO "+schema+", public"); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `CREATE TABLE issue (id uuid,workspace_id uuid,parent_issue_id uuid,project_id uuid,properties jsonb,acceptance_criteria jsonb,status text,assignee_id uuid,assignee_type text);
 CREATE TABLE agent_task_queue (issue_id uuid,status text,agent_id uuid,runtime_id uuid,context jsonb);
 CREATE TABLE issue_status (workspace_id uuid,key text,category text);`); err != nil {
		t.Fatal(err)
	}
	names := []string{"660_onespec_configuration", "661_onespec_workspace_identity", "662_onespec_project_identity", "663_onespec_flow_identity", "664_onespec_execution", "665_onespec_git_stages", "666_onespec_stage_execution", "667_onespec_execution_snapshot"}
	for _, name := range names {
		applyMigrationFile(t, ctx, conn.Conn(), name+".up.sql")
	}
	workspace, issue, agent, runtime := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err = conn.Exec(ctx, `INSERT INTO issue(id,workspace_id,status) VALUES($1,$2,'todo')`, issue, workspace); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO onespec_flow(workspace_id,issue_id,state) VALUES($1,$2,jsonb_build_object('phase','design','status','preparing','generation',0,'runtime_id',$3::text,'team',jsonb_build_object('config',jsonb_build_object('roles',jsonb_build_object('dev',jsonb_build_object('agent_id',$4::text))))))`, workspace, issue, runtime, agent); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO agent_task_queue(issue_id,status,agent_id,runtime_id,context) VALUES($1,'queued',$2,$3,'{}')`, issue, agent, runtime); err != nil {
		t.Fatal(err)
	}
	var phase string
	if err = conn.QueryRow(ctx, `SELECT context #>> '{onespec_execution,phase}' FROM agent_task_queue`).Scan(&phase); err != nil || phase != "design" {
		t.Fatalf("execution snapshot missing: %s %v", phase, err)
	}
	if _, err = conn.Exec(ctx, `UPDATE onespec_flow SET state=jsonb_set(state,'{generation}','1')`); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `UPDATE agent_task_queue SET status='running'`); err == nil {
		t.Fatal("stale queued execution was allowed")
	}
	for i := len(names) - 1; i >= 0; i-- {
		applyMigrationFile(t, ctx, conn.Conn(), names[i]+".down.sql")
	}
	for _, name := range names {
		applyMigrationFile(t, ctx, conn.Conn(), name+".up.sql")
	}
}

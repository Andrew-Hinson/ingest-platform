package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func intPtr(v int) *int { return &v }

func validSpec() projectFile {
	return projectFile{
		Project: "acme",
		Cluster: "ingest-platform",
		Tables: []table{{
			Name:    "orders",
			Columns: []column{{Name: "id", Type: "serial", PrimaryKey: true}},
		}},
	}
}

func TestParseProject_rejectsTenantIdentity(t *testing.T) {
	_, err := parseProject([]byte("tenant: acme\ncluster: ingest-platform\n"))
	if err == nil {
		t.Fatal("expected error when identity is tenant")
	}
}

func TestParseProject_rejectsTopicsAsSource(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ntopics:\n  - name: acme.public.orders\n"))
	if err == nil {
		t.Fatal("expected error when topics is source")
	}
}

func TestParseProject_rejectsACLsAsSource(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\nacls:\n  - principal: acme\n    resource: acme.\n"))
	if err == nil {
		t.Fatal("expected error when acls is source")
	}
}

func TestParseProject_rejectsConnectorsAsSource(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\nconnectors:\n  - name: acme-orders-cdc\n"))
	if err == nil {
		t.Fatal("expected error when connectors is source")
	}
}

func TestParseProject_rejectsConnectionFields(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\npassword: secret\n"))
	if err == nil {
		t.Fatal("expected error when connection fields are source")
	}
}

func TestParseProject_projectAndCluster(t *testing.T) {
	spec, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\ntables:\n  - name: orders\n    columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Project != "acme" {
		t.Fatalf("got project %q, want acme", spec.Project)
	}
	if spec.Cluster != "ingest-platform" {
		t.Fatalf("got cluster %q, want ingest-platform", spec.Cluster)
	}
}

func TestParseProject_requiresTables(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\n"))
	if err == nil {
		t.Fatal("expected error when tables is missing")
	}
}

func TestParseProject_pkCannotBeNullable(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\ntables:\n  - name: orders\n    columns:\n      - name: id\n        type: integer\n        primary_key: true\n        nullable: true\n"))
	if err == nil {
		t.Fatal("expected error when PK column is nullable")
	}
}

func TestParseProject_requiresPrimaryKey(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\ntables:\n  - name: orders\n    columns:\n      - name: amount\n        type: numeric\n"))
	if err == nil {
		t.Fatal("expected error when Table has no primary key")
	}
}

func TestParseProject_requiresColumnName(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\ntables:\n  - name: orders\n    columns:\n      - type: serial\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when column name is missing")
	}
}

func TestParseProject_namerRequiresInstanceName(t *testing.T) {
	_, err := parseProject([]byte("project: widgets\ncluster: ingest-platform\ninstance:\n  create: false\ntables:\n  - name: orders\n    database: acme\n    columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when namer omits Instance name")
	}
}

func TestParseProject_namerRequiresTableDatabase(t *testing.T) {
	_, err := parseProject([]byte("project: widgets\ncluster: ingest-platform\ninstance:\n  name: acme\ntables:\n  - name: orders\n    columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when namer Table omits database")
	}
}

func TestParseProject_instanceNameMax40(t *testing.T) {
	name := "abcdefghijklmnopqrstuvwxyz0123456789abcde"
	if len(name) != 41 {
		t.Fatalf("fixture length %d, want 41", len(name))
	}
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\n  name: " + name + "\ntables:\n  - name: orders\n    columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when Instance name is longer than 40")
	}
}

func TestParseProject_creatorProjectAsInstanceNameMax40(t *testing.T) {
	project := "abcdefghijklmnopqrstuvwxyz0123456789abcde"
	_, err := parseProject([]byte("project: " + project + "\ncluster: ingest-platform\ninstance:\n  create: true\ntables:\n  - name: orders\n    columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when creator Project used as Instance name is longer than 40")
	}
}

func TestParseProject_instanceName40Allowed(t *testing.T) {
	name := "abcdefghijklmnopqrstuvwxyz0123456789abcd"
	if len(name) != 40 {
		t.Fatalf("fixture length %d, want 40", len(name))
	}
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\n  name: " + name + "\ntables:\n  - name: orders\n    columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseProject_requiresTableName(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\ntables:\n  - columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when Table name is missing")
	}
}

func TestParseProject_rejectsUnlistedColumnType(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ninstance:\n  create: true\ntables:\n  - name: orders\n    columns:\n      - name: id\n        type: varchar\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when column type is not allowlisted")
	}
}

func TestParseProject_rejectsTopicNameOverride(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ntables:\n  - name: orders\n    topic: custom.topic\n    columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when topic name is declared")
	}
}

func TestParseProject_rejectsConnectorClassOverride(t *testing.T) {
	_, err := parseProject([]byte("project: acme\ncluster: ingest-platform\ntables:\n  - name: orders\n    class: io.confluent.example\n    columns:\n      - name: id\n        type: serial\n        primary_key: true\n"))
	if err == nil {
		t.Fatal("expected error when connector class is declared")
	}
}

func TestPlanApply_partitionsBelowDefaultFails(t *testing.T) {
	spec := validSpec()
	spec.Tables[0].Partitions = intPtr(2)
	if _, err := planApply(spec); err == nil {
		t.Fatal("expected error when partitions is below default 3")
	}
}

func TestPlanApply_partitionsMayRaise(t *testing.T) {
	spec := validSpec()
	spec.Tables[0].Partitions = intPtr(6)
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tables[0].Partitions != 6 {
		t.Fatalf("got partitions %d, want 6", plan.Tables[0].Partitions)
	}
}

func TestPlanApply_tasksMaxBelowDefaultFails(t *testing.T) {
	spec := validSpec()
	spec.Tables[0].TasksMax = intPtr(0)
	if _, err := planApply(spec); err == nil {
		t.Fatal("expected error when tasks_max is below default 1")
	}
}

func TestPlanApply_tasksMaxMayRaise(t *testing.T) {
	spec := validSpec()
	spec.Tables[0].TasksMax = intPtr(4)
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tables[0].TasksMax != 4 {
		t.Fatalf("got tasks_max %d, want 4", plan.Tables[0].TasksMax)
	}
}

func TestPlanApply_partitionsDefault(t *testing.T) {
	plan, err := planApply(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tables[0].Partitions != 3 {
		t.Fatalf("got partitions %d, want 3", plan.Tables[0].Partitions)
	}
}

func TestPlanApply_tasksMaxDefault(t *testing.T) {
	plan, err := planApply(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tables[0].TasksMax != 1 {
		t.Fatalf("got tasks_max %d, want 1", plan.Tables[0].TasksMax)
	}
}

func TestPlanApply_derivesConnectorFromTable(t *testing.T) {
	plan, err := planApply(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	c := plan.Tables[0].Connector
	if c.Name != "acme-orders-cdc" {
		t.Fatalf("got connector name %q, want acme-orders-cdc", c.Name)
	}
	if c.Class != "io.debezium.connector.postgresql.PostgresConnector" {
		t.Fatalf("got connector class %q", c.Class)
	}
	if c.Database != "acme" {
		t.Fatalf("got connector database %q, want acme", c.Database)
	}
	if c.Table != "public.orders" {
		t.Fatalf("got connector table %q, want public.orders", c.Table)
	}
	if c.TopicPrefix != "acme" {
		t.Fatalf("got connector topic prefix %q, want acme", c.TopicPrefix)
	}
}

func TestPlanApply_derivesACLFromPrefix(t *testing.T) {
	plan, err := planApply(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if plan.ACL.Principal != "acme" {
		t.Fatalf("got principal %q, want acme", plan.ACL.Principal)
	}
	if plan.ACL.Resource != "acme." {
		t.Fatalf("got ACL resource %q, want acme.", plan.ACL.Resource)
	}
	if got := strings.Join(plan.ACL.Ops, ","); got != "Read,Write,Describe" {
		t.Fatalf("got ops %q, want Read,Write,Describe", got)
	}
}

func TestPlanApply_schemaOverride(t *testing.T) {
	spec := validSpec()
	spec.Tables[0].Schema = "sales"
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tables[0].Topic != "acme.sales.orders" {
		t.Fatalf("got topic %q, want acme.sales.orders", plan.Tables[0].Topic)
	}
}

func TestPlanApply_creatorOmitsDatabasesGetsProjectDatabase(t *testing.T) {
	spec := validSpec()
	spec.Instance.Create = true
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Databases) != 1 || plan.Databases[0].Name != "acme" || !plan.Databases[0].Create {
		t.Fatalf("got databases %+v, want create acme", plan.Databases)
	}
}

func TestPlanApply_instanceNameDefaultsToProject(t *testing.T) {
	spec := validSpec()
	spec.Instance.Create = true
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Instance.Name != "acme" {
		t.Fatalf("got Instance name %q, want acme", plan.Instance.Name)
	}
	if !plan.Instance.Create {
		t.Fatal("expected Instance create")
	}
}

func TestPlanApply_secretNamedAfterInstance(t *testing.T) {
	spec := validSpec()
	spec.Instance.Create = true
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Connection.Secret != "acme" {
		t.Fatalf("got Secret %q, want acme", plan.Connection.Secret)
	}
}

func TestPlanApply_connectionFromInstanceNotYAML(t *testing.T) {
	spec := validSpec()
	spec.Instance.Create = true
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Connection.Endpoint != "acme" {
		t.Fatalf("got endpoint %q, want acme", plan.Connection.Endpoint)
	}
	if plan.Connection.Database != "acme" {
		t.Fatalf("got Database %q, want acme", plan.Connection.Database)
	}
	if plan.Connection.User != "acme" {
		t.Fatalf("got user %q, want acme", plan.Connection.User)
	}
}

func TestPlanApply_twoCreatorsSameInstanceShareSecret(t *testing.T) {
	a := validSpec()
	a.Instance.Create = true
	b := validSpec()
	b.Project = "widgets"
	b.Instance.Create = true
	b.Instance.Name = "acme"
	pa, err := planApply(a)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := planApply(b)
	if err != nil {
		t.Fatal(err)
	}
	if pa.Connection.Secret != "acme" || pb.Connection.Secret != "acme" {
		t.Fatalf("got Secrets %q and %q, want both acme", pa.Connection.Secret, pb.Connection.Secret)
	}
}

func TestPlanApply_creatorInstanceNameOverride(t *testing.T) {
	spec := validSpec()
	spec.Instance.Create = true
	spec.Instance.Name = "shared"
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Instance.Name != "shared" {
		t.Fatalf("got Instance name %q, want shared", plan.Instance.Name)
	}
	if plan.Connection.Secret != "shared" {
		t.Fatalf("got Secret %q, want shared", plan.Connection.Secret)
	}
	if plan.Connection.Endpoint != "shared" {
		t.Fatalf("got endpoint %q, want shared", plan.Connection.Endpoint)
	}
	if plan.Connection.User != "shared" {
		t.Fatalf("got user %q, want shared", plan.Connection.User)
	}
}

func TestPlanApply_generatesDDL(t *testing.T) {
	spec := validSpec()
	nullableFalse := false
	spec.Tables[0].Columns = []column{
		{Name: "id", Type: "serial", PrimaryKey: true},
		{Name: "user_id", Type: "integer", Nullable: &nullableFalse},
	}
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	want := "CREATE TABLE IF NOT EXISTS public.orders (\n  id SERIAL PRIMARY KEY,\n  user_id INTEGER NOT NULL\n)"
	if plan.Tables[0].DDL != want {
		t.Fatalf("got DDL:\n%s\nwant:\n%s", plan.Tables[0].DDL, want)
	}
}

func TestPlanApply_derivesTopicFromTable(t *testing.T) {
	plan, err := planApply(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tables[0].Topic != "acme.public.orders" {
		t.Fatalf("got topic %q, want acme.public.orders", plan.Tables[0].Topic)
	}
}

func TestPlanApply_eachTableGetsTopicAndConnector(t *testing.T) {
	spec := validSpec()
	spec.Tables = append(spec.Tables, table{
		Name:    "items",
		Columns: []column{{Name: "id", Type: "serial", PrimaryKey: true}},
	})
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tables) != 2 {
		t.Fatalf("got %d tables, want 2", len(plan.Tables))
	}
	if plan.Tables[0].Topic != "acme.public.orders" {
		t.Fatalf("got topic %q, want acme.public.orders", plan.Tables[0].Topic)
	}
	if plan.Tables[1].Topic != "acme.public.items" {
		t.Fatalf("got topic %q, want acme.public.items", plan.Tables[1].Topic)
	}
	if plan.Tables[1].Connector.Name != "acme-items-cdc" {
		t.Fatalf("got connector %q, want acme-items-cdc", plan.Tables[1].Connector.Name)
	}
}

func TestPlanApply_exampleProjectYAML(t *testing.T) {
	tfDir, err := findTFDir()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(tfDir), "examples", "acme.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(string(raw)), "demo") {
		t.Fatal("example YAML must be labeled a demo")
	}
	spec, err := parseProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !spec.Instance.Create {
		t.Fatal("example YAML must create the Instance")
	}
	if len(spec.Tables) == 0 || len(spec.Tables[0].Columns) == 0 {
		t.Fatal("example YAML must declare Table columns")
	}
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Prefix != "acme" {
		t.Fatalf("got prefix %q, want acme", plan.Prefix)
	}
	if plan.Tables[0].Topic != "acme.public.orders" {
		t.Fatalf("got topic %q, want acme.public.orders", plan.Tables[0].Topic)
	}
	if plan.ACL.Resource != "acme." {
		t.Fatalf("got ACL resource %q, want acme.", plan.ACL.Resource)
	}
	if plan.Tables[0].Connector.Name != "acme-orders-cdc" {
		t.Fatalf("got connector %q, want acme-orders-cdc", plan.Tables[0].Connector.Name)
	}
	if plan.Tables[0].Connector.Database != "acme" {
		t.Fatalf("got connector database %q, want acme", plan.Tables[0].Connector.Database)
	}
	if plan.Instance.Name != "acme" || !plan.Instance.Create {
		t.Fatalf("got Instance %+v, want create acme", plan.Instance)
	}
	if plan.Connection.Secret != "acme" {
		t.Fatalf("got Secret %q, want acme", plan.Connection.Secret)
	}
	if plan.Connection.Endpoint != "acme" {
		t.Fatalf("got endpoint %q, want acme", plan.Connection.Endpoint)
	}
}

func TestPlanApply_prefixDefaultsToProject(t *testing.T) {
	spec := validSpec()
	spec.Prefix = ""
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tables[0].Topic != "acme.public.orders" {
		t.Fatalf("got topic %q, want acme.public.orders", plan.Tables[0].Topic)
	}
}

func TestPlanApply_prefixOverride(t *testing.T) {
	spec := validSpec()
	spec.Prefix = "widgets"
	plan, err := planApply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tables[0].Topic != "widgets.public.orders" {
		t.Fatalf("got topic %q, want widgets.public.orders", plan.Tables[0].Topic)
	}
}

func TestFindTFDir_findsTF(t *testing.T) {
	root := t.TempDir()
	tfDir := filepath.Join(root, "tf")
	if err := os.MkdirAll(tfDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	got, err := findTFDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != tfDir {
		t.Fatalf("got %q, want %q", got, tfDir)
	}
}

func TestFindTFDir_rejectsLab(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lab", "tf"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if _, err := findTFDir(); err == nil {
		t.Fatal("expected error when only lab/tf exists")
	}
}

func TestProjectStateDir_twoProjectsDoNotShareState(t *testing.T) {
	root := t.TempDir()
	acme := projectStateDir(root, "acme")
	bravo := projectStateDir(root, "bravo")
	if acme == bravo {
		t.Fatal("acme and bravo must not share Apply state")
	}
	wantAcme := filepath.Join(root, ".ingestctl", "acme")
	wantBravo := filepath.Join(root, ".ingestctl", "bravo")
	if acme != wantAcme {
		t.Fatalf("got %q, want %q", acme, wantAcme)
	}
	if bravo != wantBravo {
		t.Fatalf("got %q, want %q", bravo, wantBravo)
	}
}

func TestWriteApplyFiles_twoProjectsDoNotClobberTfvars(t *testing.T) {
	root := t.TempDir()
	acmeDir := projectStateDir(root, "acme")
	bravoDir := projectStateDir(root, "bravo")
	acmeVars, acmeState, err := writeApplyFiles(acmeDir, "topic_name = \"acme.public.orders\"\n")
	if err != nil {
		t.Fatal(err)
	}
	bravoVars, bravoState, err := writeApplyFiles(bravoDir, "topic_name = \"bravo.public.orders\"\n")
	if err != nil {
		t.Fatal(err)
	}
	acmeRaw, err := os.ReadFile(acmeVars)
	if err != nil {
		t.Fatal(err)
	}
	bravoRaw, err := os.ReadFile(bravoVars)
	if err != nil {
		t.Fatal(err)
	}
	if string(acmeRaw) != "topic_name = \"acme.public.orders\"\n" {
		t.Fatalf("acme tfvars clobbered: %s", acmeRaw)
	}
	if string(bravoRaw) != "topic_name = \"bravo.public.orders\"\n" {
		t.Fatalf("bravo tfvars clobbered: %s", bravoRaw)
	}
	if acmeState == bravoState {
		t.Fatal("acme and bravo must not share terraform state path")
	}
}

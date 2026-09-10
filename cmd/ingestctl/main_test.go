package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validSpec() projectFile {
	return projectFile{
		Project: "acme",
		Cluster: "kind",
		Topics:  []topic{{Name: "acme.public.orders", Partitions: 6, Replication: 3}},
		ACLs:    []acl{{Principal: "acme", Resource: "acme.", Ops: []string{"Read", "Write", "Describe"}}},
		Connectors: []connector{{
			Name:        "acme-orders-cdc",
			Class:       "io.debezium.connector.postgresql.PostgresConnector",
			Database:    "acme",
			Table:       "public.orders",
			TopicPrefix: "acme",
		}},
	}
}

func TestParseProject_rejectsTenantIdentity(t *testing.T) {
	_, err := parseProject([]byte("tenant: acme\ncluster: kind\n"))
	if err == nil {
		t.Fatal("expected error when identity is tenant")
	}
}

func TestParseProject_projectAndCluster(t *testing.T) {
	spec, err := parseProject([]byte("project: acme\ncluster: kind\n"))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Project != "acme" {
		t.Fatalf("got project %q, want acme", spec.Project)
	}
	if spec.Cluster != "kind" {
		t.Fatalf("got cluster %q, want kind", spec.Cluster)
	}
}

func TestPlanApply_exampleProjectYAML(t *testing.T) {
	tfDir, err := findTFDir()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(tfDir)), "examples", "acme.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := parseProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := planApply(spec, 2)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Prefix != "acme" {
		t.Fatalf("got prefix %q, want acme", spec.Prefix)
	}
	want := []string{
		`topic_name = "acme.public.orders"`,
		`acl_resource = "acme."`,
		`connector_name = "acme-orders-cdc"`,
		`connector_database = "acme"`,
	}
	if strings.Contains(out, "\nprefix = ") || strings.HasPrefix(out, "prefix = ") {
		t.Fatalf("ACL resource must not be tfvars prefix; glossary Prefix is YAML prefix:\n%s", out)
	}
	for _, line := range want {
		if !strings.Contains(out, line) {
			t.Fatalf("missing %s in:\n%s", line, out)
		}
	}
}

func TestPlanApply_opsPassThrough(t *testing.T) {
	spec := validSpec()
	spec.ACLs[0].Ops = []string{"Read"}
	out, err := planApply(spec, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `topic_ops = ["Read"]`) {
		t.Fatalf("expected Read-only topic_ops, got:\n%s", out)
	}
	if strings.Contains(out, `"Write"`) {
		t.Fatalf("Read-only ops must not grant Write:\n%s", out)
	}
}

func TestPlanApply_rfBelowMinISR(t *testing.T) {
	spec := validSpec()
	spec.Topics[0].Replication = 1
	if _, err := planApply(spec, 2); err == nil {
		t.Fatal("expected error when replication is below min-ISR")
	}
}

func TestPlanApply_prefixDefaultsToProject(t *testing.T) {
	spec := validSpec()
	spec.Prefix = ""
	spec.Topics[0].Name = "public.orders"
	out, err := planApply(spec, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `topic_name = "acme.public.orders"`) {
		t.Fatalf("expected template {prefix}.{name} with prefix=project, got:\n%s", out)
	}
}

func TestPlanApply_prefixOverride(t *testing.T) {
	spec := validSpec()
	spec.Prefix = "widgets"
	spec.Topics[0].Name = "public.orders"
	out, err := planApply(spec, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `topic_name = "widgets.public.orders"`) {
		t.Fatalf("expected override prefix in template, got:\n%s", out)
	}
	if strings.Contains(out, `topic_name = "acme.public.orders"`) {
		t.Fatalf("project name must not win over Prefix override:\n%s", out)
	}
}

func TestPlanApply_connectorFields(t *testing.T) {
	out, err := planApply(validSpec(), 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`connector_name = "acme-orders-cdc"`,
		`connector_class = "io.debezium.connector.postgresql.PostgresConnector"`,
		`connector_database = "acme"`,
		`connector_table = "public.orders"`,
		`connector_topic_prefix = "acme"`,
	}
	for _, line := range want {
		if !strings.Contains(out, line) {
			t.Fatalf("missing %s in:\n%s", line, out)
		}
	}
	if strings.Contains(out, `connector_database = "lab"`) {
		t.Fatalf("connector must use database acme, not lab:\n%s", out)
	}
}

func TestPlanApply_exactlyOneConnector(t *testing.T) {
	none := validSpec()
	none.Connectors = nil
	if _, err := planApply(none, 2); err == nil {
		t.Fatal("expected error when connectors is empty")
	}
	two := validSpec()
	two.Connectors = []connector{{Name: "a"}, {Name: "b"}}
	if _, err := planApply(two, 2); err == nil {
		t.Fatal("expected error when two connectors are given")
	}
}

func TestFindTFDir_kindNotLab(t *testing.T) {
	root := t.TempDir()
	kindTF := filepath.Join(root, "kind", "tf")
	if err := os.MkdirAll(kindTF, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	got, err := findTFDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != kindTF {
		t.Fatalf("got %q, want %q", got, kindTF)
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

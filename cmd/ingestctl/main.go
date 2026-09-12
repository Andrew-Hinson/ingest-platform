// ingestctl apply -f examples/acme.yaml
// Parses Project YAML, writes per-Project local Terraform state, runs terraform init/apply.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type projectFile struct {
	Project   string       `yaml:"project"`
	Cluster   string       `yaml:"cluster"`
	Prefix    string       `yaml:"prefix"`
	Instance  instanceSpec `yaml:"instance"`
	Databases []string     `yaml:"databases"`
	Tables    []table      `yaml:"tables"`
}

type instanceSpec struct {
	Create bool   `yaml:"create"`
	Name   string `yaml:"name"`
}

type table struct {
	Name       string   `yaml:"name"`
	Schema     string   `yaml:"schema"`
	Database   string   `yaml:"database"`
	Partitions *int     `yaml:"partitions"`
	TasksMax   *int     `yaml:"tasks_max"`
	Columns    []column `yaml:"columns"`
}

type column struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Nullable   *bool  `yaml:"nullable"`
	PrimaryKey bool   `yaml:"primary_key"`
}

// parseProject decodes Project YAML and rejects unknown keys or an invalid Table contract.
func parseProject(raw []byte) (projectFile, error) {
	var spec projectFile
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil {
		return spec, err
	}
	if spec.Project == "" {
		return spec, errors.New("project is required")
	}
	if spec.Cluster == "" {
		return spec, errors.New("cluster is required")
	}
	if !spec.Instance.Create && spec.Instance.Name == "" {
		return spec, errors.New("namer instance requires name")
	}
	instName := spec.Instance.Name
	if spec.Instance.Create && instName == "" {
		instName = spec.Project
	}
	if len(instName) > 40 {
		return spec, errors.New("instance name must be at most 40 characters")
	}
	if len(spec.Tables) == 0 {
		return spec, errors.New("tables is required")
	}
	for _, tbl := range spec.Tables {
		if tbl.Name == "" {
			return spec, errors.New("table name is required")
		}
		if !spec.Instance.Create && tbl.Database == "" {
			return spec, errors.New("namer table requires database")
		}
		hasPK := false
		for _, col := range tbl.Columns {
			if col.Name == "" {
				return spec, errors.New("column name is required")
			}
			if !allowedColumnTypes[col.Type] {
				return spec, fmt.Errorf("column type %q is not allowed", col.Type)
			}
			if col.PrimaryKey {
				hasPK = true
				if col.Nullable != nil && *col.Nullable {
					return spec, errors.New("primary key cannot be nullable")
				}
			}
		}
		if !hasPK {
			return spec, errors.New("table requires a primary key")
		}
	}
	return spec, nil
}

var allowedColumnTypes = map[string]bool{
	"integer":     true,
	"bigint":      true,
	"text":        true,
	"numeric":     true,
	"boolean":     true,
	"timestamptz": true,
	"serial":      true,
}

type applyPlan struct {
	Project    string
	Prefix     string
	Instance   plannedInstance
	Databases  []plannedDatabase
	Connection plannedConnection
	ACL        plannedACL
	Tables     []plannedTable
}

type plannedDatabase struct {
	Name   string
	Create bool
}

type plannedInstance struct {
	Name   string
	Create bool
}

type plannedConnection struct {
	Endpoint string
	Database string
	User     string
	Secret   string
}

type plannedACL struct {
	Principal string
	Resource  string
	Ops       []string
}

type plannedTable struct {
	Name       string
	Schema     string
	Database   string
	Topic      string
	Partitions int
	TasksMax   int
	DDL        string
	Connector  plannedConnector
}

type plannedConnector struct {
	Name        string
	Class       string
	Database    string
	Table       string
	TopicPrefix string
}

// main runs apply and exits 1 on error.
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run parses YAML, plans Apply, writes per-Project state, and terraform-applies tf/.
func run(args []string) error {
	if len(args) == 0 || args[0] != "apply" {
		return errors.New("usage: ingestctl apply -f <project.yaml>")
	}

	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("f", "", "Project YAML")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("usage: ingestctl apply -f <project.yaml>")
	}

	raw, err := readYAML(*file)
	if err != nil {
		return err
	}
	spec, err := parseProject(raw)
	if err != nil {
		return err
	}
	tfDir, err := findTFDir()
	if err != nil {
		return err
	}
	plan, err := planApply(spec)
	if err != nil {
		return err
	}
	root := filepath.Dir(tfDir)
	stateDir := projectStateDir(root, spec.Project)
	tfvarsPath, statePath, err := writeApplyFiles(stateDir, renderTfvars(plan))
	if err != nil {
		return err
	}

	if err := runTerraform(tfDir, "init"); err != nil {
		return err
	}
	if err := runTerraform(tfDir, "apply", "-auto-approve", "-state="+statePath, "-var-file="+tfvarsPath); err != nil {
		return err
	}
	fmt.Print(formatConnection(plan.Connection))
	return nil
}

// planApply derives Instance/Database actions, Kafka names, and DDL from each Table.
func planApply(spec projectFile) (applyPlan, error) {
	var plan applyPlan
	if len(spec.Tables) == 0 {
		return plan, errors.New("tables is required")
	}
	prefix := spec.Prefix
	if prefix == "" {
		prefix = spec.Project
	}
	instName := spec.Instance.Name
	if spec.Instance.Create && instName == "" {
		instName = spec.Project
	}
	plan.Project = spec.Project
	plan.Prefix = prefix
	plan.Instance = plannedInstance{Name: instName, Create: spec.Instance.Create}
	if spec.Instance.Create && len(spec.Databases) == 0 {
		plan.Databases = []plannedDatabase{{Name: spec.Project, Create: true}}
	} else {
		for _, name := range spec.Databases {
			plan.Databases = append(plan.Databases, plannedDatabase{Name: name, Create: true})
		}
	}
	database := spec.Project
	if len(plan.Databases) > 0 {
		database = plan.Databases[0].Name
	}
	plan.Connection = plannedConnection{
		Endpoint: instName,
		Database: database,
		User:     instName,
		Secret:   instName,
	}
	plan.ACL = plannedACL{
		Principal: prefix,
		Resource:  prefix + ".",
		Ops:       []string{"Read", "Write", "Describe"},
	}
	for _, t := range spec.Tables {
		schema := t.Schema
		if schema == "" {
			schema = "public"
		}
		database := t.Database
		if database == "" {
			database = spec.Project
		}
		partitions := defaultPartitions
		if t.Partitions != nil {
			partitions = *t.Partitions
		}
		if partitions < defaultPartitions {
			return plan, fmt.Errorf("partitions %d is below default %d", partitions, defaultPartitions)
		}
		tasksMax := defaultTasksMax
		if t.TasksMax != nil {
			tasksMax = *t.TasksMax
		}
		if tasksMax < defaultTasksMax {
			return plan, fmt.Errorf("tasks_max %d is below default %d", tasksMax, defaultTasksMax)
		}
		plan.Tables = append(plan.Tables, plannedTable{
			Name:       t.Name,
			Schema:     schema,
			Database:   database,
			Topic:      prefix + "." + schema + "." + t.Name,
			Partitions: partitions,
			TasksMax:   tasksMax,
			DDL:        tableDDL(schema, t.Name, t.Columns),
			Connector: plannedConnector{
				Name:        prefix + "-" + t.Name + "-cdc",
				Class:       debeziumPostgresClass,
				Database:    database,
				Table:       schema + "." + t.Name,
				TopicPrefix: prefix,
			},
		})
	}
	return plan, nil
}

// tableDDL builds CREATE TABLE IF NOT EXISTS from columns and the primary key.
func tableDDL(schema, name string, cols []column) string {
	var pks []string
	for _, col := range cols {
		if col.PrimaryKey {
			pks = append(pks, col.Name)
		}
	}
	inlinePK := len(pks) == 1
	var b strings.Builder
	b.WriteString("CREATE TABLE IF NOT EXISTS ")
	b.WriteString(schema)
	b.WriteByte('.')
	b.WriteString(name)
	b.WriteString(" (\n")
	for i, col := range cols {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString("  ")
		b.WriteString(col.Name)
		b.WriteByte(' ')
		b.WriteString(sqlColumnTypes[col.Type])
		if col.PrimaryKey && inlinePK {
			b.WriteString(" PRIMARY KEY")
		} else if !columnNullable(col) {
			b.WriteString(" NOT NULL")
		}
	}
	if !inlinePK {
		b.WriteString(",\n  PRIMARY KEY (")
		b.WriteString(strings.Join(pks, ", "))
		b.WriteString(")")
	}
	b.WriteString("\n)")
	return b.String()
}

// columnNullable is false for PK columns; others default true unless YAML sets nullable.
func columnNullable(col column) bool {
	if col.PrimaryKey {
		return false
	}
	if col.Nullable == nil {
		return true
	}
	return *col.Nullable
}

var sqlColumnTypes = map[string]string{
	"integer":     "INTEGER",
	"bigint":      "BIGINT",
	"text":        "TEXT",
	"numeric":     "NUMERIC",
	"boolean":     "BOOLEAN",
	"timestamptz": "TIMESTAMPTZ",
	"serial":      "SERIAL",
}

// renderTfvars writes terraform vars for the Instance and first planned Table.
func renderTfvars(plan applyPlan) string {
	t := plan.Tables[0]
	var b strings.Builder
	writeStr(&b, "instance_name", plan.Instance.Name)
	writeBool(&b, "instance_create", plan.Instance.Create)
	writeStr(&b, "instance_database", plan.Connection.Database)
	writeStr(&b, "instance_creator", plan.Project)
	writeStr(&b, "topic_name", t.Topic)
	writeNum(&b, "partitions", t.Partitions)
	writeNum(&b, "replicas", topicReplicas)
	writeNum(&b, "min_insync_replicas", topicMinISR)
	writeStr(&b, "principal", plan.ACL.Principal)
	writeStr(&b, "acl_resource", plan.ACL.Resource)
	writeList(&b, "topic_ops", plan.ACL.Ops)
	writeStr(&b, "connector_name", t.Connector.Name)
	writeStr(&b, "connector_class", t.Connector.Class)
	writeStr(&b, "connector_database", t.Connector.Database)
	writeStr(&b, "connector_table", t.Connector.Table)
	writeStr(&b, "connector_topic_prefix", t.Connector.TopicPrefix)
	writeStr(&b, "connector_hostname", plan.Instance.Name)
	return b.String()
}

const (
	debeziumPostgresClass = "io.debezium.connector.postgresql.PostgresConnector"
	defaultPartitions     = 3
	defaultTasksMax       = 1
	topicReplicas         = 3
	topicMinISR           = 2
)

// projectStateDir is the per-Project Apply state path under .ingestctl.
func projectStateDir(root, project string) string {
	return filepath.Join(root, ".ingestctl", project)
}

// writeApplyFiles writes terraform.tfvars and returns the sibling state path.
func writeApplyFiles(stateDir, tfvars string) (tfvarsPath, statePath string, err error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", "", err
	}
	tfvarsPath = filepath.Join(stateDir, "terraform.tfvars")
	statePath = filepath.Join(stateDir, "terraform.tfstate")
	if err := os.WriteFile(tfvarsPath, []byte(tfvars), 0644); err != nil {
		return "", "", err
	}
	return tfvarsPath, statePath, nil
}

// writeBool appends a tfvars bool assignment.
func writeBool(b *strings.Builder, key string, v bool) {
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.FormatBool(v))
	b.WriteByte('\n')
}

// writeStr appends a quoted tfvars string assignment.
func writeStr(b *strings.Builder, key, v string) {
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.Quote(v))
	b.WriteByte('\n')
}

// writeNum appends a tfvars number assignment.
func writeNum(b *strings.Builder, key string, v int) {
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.Itoa(v))
	b.WriteByte('\n')
}

// writeList appends a tfvars list of quoted strings.
func writeList(b *strings.Builder, key string, vs []string) {
	b.WriteString(key)
	b.WriteString(" = [")
	for i, v := range vs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(v))
	}
	b.WriteString("]\n")
}

// readYAML reads path, or the same path from the repo root if the first read fails.
func readYAML(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		return raw, nil
	}
	tfDir, ferr := findTFDir()
	if ferr != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(filepath.Dir(tfDir), path))
}

// findTFDir walks up from cwd until it finds tf/.
func findTFDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, "tf")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("tf/ not found; run from the repo")
		}
		dir = parent
	}
}

// formatConnection prints endpoint, Database, user, and Secret name. Password is never printed.
func formatConnection(conn plannedConnection) string {
	return "endpoint: " + conn.Endpoint + "\n" +
		"database: " + conn.Database + "\n" +
		"user: " + conn.User + "\n" +
		"secret: " + conn.Secret + "\n"
}

// runTerraform runs terraform with args in dir.
func runTerraform(dir string, args ...string) error {
	cmd := exec.Command("terraform", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

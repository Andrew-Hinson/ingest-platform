// ingestctl apply -f examples/acme.yaml
// Parses Project YAML, writes per-Project local Terraform state, runs terraform init/apply.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
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

type liveSnapshot struct {
	Databases []string
	Tables    []liveTable
}

type liveTable struct {
	Database string
	Schema   string
	Name     string
	Columns  []liveColumn
}

type liveColumn struct {
	Name       string
	Type       string
	Nullable   bool
	PrimaryKey bool
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
	DDL    string
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
	Publication string
}

// main runs apply and exits 1 on error.
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run parses YAML, plans Apply against live DDL, writes per-Project state, applies SQL, then terraform.
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
	user, password, err := retrieveInstanceSecret(instanceName(spec))
	if err != nil {
		return err
	}
	login := instanceLogin{Host: instanceName(spec), User: user, Password: password}
	live, err := inspectLive(login)
	if err != nil {
		return err
	}
	plan, err := planApply(spec, live)
	if err != nil {
		return err
	}
	plan.Connection.User = user
	root := filepath.Dir(tfDir)
	stateDir := projectStateDir(root, spec.Project)
	tfvarsPath, statePath, _, err := writeApplyFiles(stateDir, renderTfvars(plan), renderApplySQL(plan))
	if err != nil {
		return err
	}
	if err := applySQL(login, plan); err != nil {
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
// live is the Instance snapshot; a present Table that does not match YAML is an error.
func planApply(spec projectFile, live liveSnapshot) (applyPlan, error) {
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
		plan.Databases = []plannedDatabase{ownedDatabase(spec.Project, live)}
	} else {
		for _, name := range spec.Databases {
			plan.Databases = append(plan.Databases, ownedDatabase(name, live))
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
		if liveTbl, ok := findLiveTable(live, database, schema, t.Name); ok {
			if !tableMatchesYAML(t.Columns, liveTbl.Columns) {
				return plan, fmt.Errorf("table %s.%s does not match YAML DDL", schema, t.Name)
			}
		}
		connectorName := prefix + "-" + t.Name + "-cdc"
		plan.Tables = append(plan.Tables, plannedTable{
			Name:       t.Name,
			Schema:     schema,
			Database:   database,
			Topic:      prefix + "." + schema + "." + t.Name,
			Partitions: partitions,
			TasksMax:   tasksMax,
			DDL:        tableDDL(schema, t.Name, t.Columns),
			Connector: plannedConnector{
				Name:        connectorName,
				Class:       debeziumPostgresClass,
				Database:    database,
				Table:       schema + "." + t.Name,
				TopicPrefix: prefix,
				Publication: strings.ReplaceAll(connectorName, "-", "_"),
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

// ownedDatabase is a Database this Project creates. DDL is omitted when live already has it.
func ownedDatabase(name string, live liveSnapshot) plannedDatabase {
	db := plannedDatabase{Name: name, Create: true}
	if !liveHasDatabase(live, name) {
		db.DDL = "CREATE DATABASE " + name
	}
	return db
}

// liveHasDatabase reports whether the snapshot lists name.
func liveHasDatabase(live liveSnapshot, name string) bool {
	for _, db := range live.Databases {
		if db == name {
			return true
		}
	}
	return false
}

// findLiveTable returns the snapshot row for database.schema.name.
func findLiveTable(live liveSnapshot, database, schema, name string) (liveTable, bool) {
	for _, tbl := range live.Tables {
		if tbl.Database == database && tbl.Schema == schema && tbl.Name == name {
			return tbl, true
		}
	}
	return liveTable{}, false
}

// tableMatchesYAML is true when live columns match YAML name, type, nullability, and PK.
func tableMatchesYAML(want []column, live []liveColumn) bool {
	if len(want) != len(live) {
		return false
	}
	byName := make(map[string]liveColumn, len(live))
	for _, col := range live {
		byName[col.Name] = col
	}
	for _, col := range want {
		got, ok := byName[col.Name]
		if !ok {
			return false
		}
		if got.Type != col.Type || got.Nullable != columnNullable(col) || got.PrimaryKey != col.PrimaryKey {
			return false
		}
	}
	return true
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

// renderTfvars writes terraform vars for the Instance and every planned Table.
func renderTfvars(plan applyPlan) string {
	var b strings.Builder
	writeStr(&b, "instance_name", plan.Instance.Name)
	writeBool(&b, "instance_create", plan.Instance.Create)
	writeStr(&b, "instance_database", plan.Connection.Database)
	writeStr(&b, "instance_creator", plan.Project)
	writeNum(&b, "replicas", topicReplicas)
	writeNum(&b, "min_insync_replicas", topicMinISR)
	writeStr(&b, "principal", plan.ACL.Principal)
	writeStr(&b, "acl_resource", plan.ACL.Resource)
	writeList(&b, "topic_ops", plan.ACL.Ops)
	writeStr(&b, "connector_hostname", plan.Instance.Name)
	b.WriteString("tables = [\n")
	for _, t := range plan.Tables {
		b.WriteString("  {\n")
		writeStrIndent(&b, "    ", "topic_name", t.Topic)
		writeNumIndent(&b, "    ", "partitions", t.Partitions)
		writeStrIndent(&b, "    ", "connector_name", t.Connector.Name)
		writeStrIndent(&b, "    ", "connector_class", t.Connector.Class)
		writeStrIndent(&b, "    ", "connector_database", t.Connector.Database)
		writeStrIndent(&b, "    ", "connector_table", t.Connector.Table)
		writeStrIndent(&b, "    ", "connector_topic_prefix", t.Connector.TopicPrefix)
		writeNumIndent(&b, "    ", "tasks_max", t.TasksMax)
		writeStrIndent(&b, "    ", "publication_name", t.Connector.Publication)
		b.WriteString("  },\n")
	}
	b.WriteString("]\n")
	return b.String()
}

const (
	debeziumPostgresClass = "io.debezium.connector.postgresql.PostgresConnector"
	defaultPartitions     = 3
	defaultTasksMax       = 1
	topicReplicas         = 3
	topicMinISR           = 2
	postgresPort          = "5432"
)

type instanceLogin struct {
	Host     string
	User     string
	Password string
}

// projectStateDir is the per-Project Apply state path under .ingestctl.
func projectStateDir(root, project string) string {
	return filepath.Join(root, ".ingestctl", project)
}

// writeApplyFiles writes terraform.tfvars and apply.sql under the Project state dir.
func writeApplyFiles(stateDir, tfvars, sql string) (tfvarsPath, statePath, sqlPath string, err error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", "", "", err
	}
	tfvarsPath = filepath.Join(stateDir, "terraform.tfvars")
	statePath = filepath.Join(stateDir, "terraform.tfstate")
	sqlPath = filepath.Join(stateDir, "apply.sql")
	if err := os.WriteFile(tfvarsPath, []byte(tfvars), 0644); err != nil {
		return "", "", "", err
	}
	if err := os.WriteFile(sqlPath, []byte(sql), 0644); err != nil {
		return "", "", "", err
	}
	return tfvarsPath, statePath, sqlPath, nil
}

// renderApplySQL concatenates owned Database and Table DDL. It is Apply output, not source.
func renderApplySQL(plan applyPlan) string {
	var b strings.Builder
	for _, db := range plan.Databases {
		if db.DDL == "" {
			continue
		}
		b.WriteString(db.DDL)
		b.WriteByte('\n')
	}
	for _, tbl := range plan.Tables {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(tbl.DDL)
		b.WriteByte('\n')
	}
	return b.String()
}

// writeStrIndent appends an indented quoted tfvars string assignment.
func writeStrIndent(b *strings.Builder, indent, key, v string) {
	b.WriteString(indent)
	writeStr(b, key, v)
}

// writeNumIndent appends an indented tfvars number assignment.
func writeNumIndent(b *strings.Builder, indent, key string, v int) {
	b.WriteString(indent)
	writeNum(b, key, v)
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

// instanceName is the logical Instance name from YAML (creator default is Project).
func instanceName(spec projectFile) string {
	if spec.Instance.Name != "" {
		return spec.Instance.Name
	}
	return spec.Project
}

// retrieveInstanceSecret loads user and password from Secrets Manager. Apply does not create it.
func retrieveInstanceSecret(name string) (user, password string, err error) {
	out, err := exec.Command("aws", "secretsmanager", "get-secret-value", "--secret-id", name, "--query", "SecretString", "--output", "text").Output()
	if err != nil {
		return "", "", fmt.Errorf("instance secret %s: %w", name, err)
	}
	var creds struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(out, &creds); err != nil {
		return "", "", err
	}
	if creds.User == "" || creds.Password == "" {
		return "", "", fmt.Errorf("instance secret %s missing user or password", name)
	}
	return creds.User, creds.Password, nil
}

// inspectLive reads Databases and Table columns from the Instance.
func inspectLive(login instanceLogin) (liveSnapshot, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, postgresURL(login, "postgres"))
	if err != nil {
		return liveSnapshot{}, err
	}
	defer conn.Close(ctx)
	rows, err := conn.Query(ctx, `SELECT datname FROM pg_database WHERE datistemplate = false`)
	if err != nil {
		return liveSnapshot{}, err
	}
	defer rows.Close()
	var live liveSnapshot
	var dbs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return liveSnapshot{}, err
		}
		dbs = append(dbs, name)
	}
	if err := rows.Err(); err != nil {
		return liveSnapshot{}, err
	}
	live.Databases = dbs
	for _, db := range dbs {
		if db == "postgres" || db == "rdsadmin" {
			continue
		}
		tables, err := inspectDatabase(ctx, login, db)
		if err != nil {
			return liveSnapshot{}, err
		}
		live.Tables = append(live.Tables, tables...)
	}
	return live, nil
}

// inspectDatabase reads Table columns in one Database.
func inspectDatabase(ctx context.Context, login instanceLogin, database string) ([]liveTable, error) {
	conn, err := pgx.Connect(ctx, postgresURL(login, database))
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)
	rows, err := conn.Query(ctx, `
SELECT n.nspname, c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'r' AND n.nspname NOT IN ('pg_catalog', 'information_schema')`)
	if err != nil {
		return nil, err
	}
	var found []liveTable
	for rows.Next() {
		var schema, name string
		if err := rows.Scan(&schema, &name); err != nil {
			rows.Close()
			return nil, err
		}
		found = append(found, liveTable{Database: database, Schema: schema, Name: name})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range found {
		cols, err := inspectColumns(ctx, conn, found[i].Schema, found[i].Name)
		if err != nil {
			return nil, err
		}
		found[i].Columns = cols
	}
	return found, nil
}

// inspectColumns reads name, mapped type, nullability, and PK for one Table.
func inspectColumns(ctx context.Context, conn *pgx.Conn, schema, name string) ([]liveColumn, error) {
	rel := schema + "." + name
	pkRows, err := conn.Query(ctx, `
SELECT a.attname
FROM pg_index i
JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY (i.indkey)
WHERE i.indrelid = $1::regclass AND i.indisprimary`, rel)
	if err != nil {
		return nil, err
	}
	pks := map[string]bool{}
	for pkRows.Next() {
		var col string
		if err := pkRows.Scan(&col); err != nil {
			pkRows.Close()
			return nil, err
		}
		pks[col] = true
	}
	if err := pkRows.Err(); err != nil {
		pkRows.Close()
		return nil, err
	}
	pkRows.Close()
	rows, err := conn.Query(ctx, `
SELECT a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod), NOT a.attnotnull, COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '')
FROM pg_attribute a
LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
WHERE a.attrelid = $1::regclass AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY a.attnum`, rel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []liveColumn
	for rows.Next() {
		var col liveColumn
		var pgType, def string
		if err := rows.Scan(&col.Name, &pgType, &col.Nullable, &def); err != nil {
			return nil, err
		}
		col.Type = mapLiveType(pgType, def)
		col.PrimaryKey = pks[col.Name]
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

// mapLiveType maps a Postgres type (and default) onto the YAML allowlist.
func mapLiveType(pgType, def string) string {
	base := pgType
	if i := strings.IndexByte(pgType, '('); i >= 0 {
		base = pgType[:i]
	}
	if strings.HasPrefix(def, "nextval(") && base == "integer" {
		return "serial"
	}
	if base == "timestamp with time zone" {
		return "timestamptz"
	}
	return base
}

// applySQL runs owned Database and Table DDL on the Instance.
func applySQL(login instanceLogin, plan applyPlan) error {
	ctx := context.Background()
	for _, db := range plan.Databases {
		if db.DDL == "" {
			continue
		}
		if err := execSQL(ctx, login, "postgres", db.DDL); err != nil {
			return err
		}
	}
	for _, tbl := range plan.Tables {
		if err := execSQL(ctx, login, tbl.Database, tbl.DDL); err != nil {
			return err
		}
	}
	return nil
}

// execSQL runs one statement against database.
func execSQL(ctx context.Context, login instanceLogin, database, sql string) error {
	conn, err := pgx.Connect(ctx, postgresURL(login, database))
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, sql)
	return err
}

// postgresURL is a libpq URL for host:5432. Password is not logged.
func postgresURL(login instanceLogin, database string) string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(login.User, login.Password),
		Host:   net.JoinHostPort(login.Host, postgresPort),
		Path:   "/" + database,
	}
	q := u.Query()
	q.Set("sslmode", "prefer")
	u.RawQuery = q.Encode()
	return u.String()
}

// runTerraform runs terraform with args in dir.
func runTerraform(dir string, args ...string) error {
	cmd := exec.Command("terraform", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

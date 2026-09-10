// ingestctl apply -f examples/acme.yaml
// Parses Project YAML, writes per-Project local Terraform state, runs terraform init/apply.
package main

import (
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
	Tenant     string      `yaml:"tenant"`
	Project    string      `yaml:"project"`
	Cluster    string      `yaml:"cluster"`
	Prefix     string      `yaml:"prefix"`
	Topics     []topic     `yaml:"topics"`
	ACLs       []acl       `yaml:"acls"`
	Connectors []connector `yaml:"connectors"`
}

func parseProject(raw []byte) (projectFile, error) {
	var spec projectFile
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return spec, err
	}
	if spec.Tenant != "" {
		return spec, errors.New("tenant is not a Project identity")
	}
	if spec.Project == "" {
		return spec, errors.New("project is required")
	}
	if spec.Cluster == "" {
		return spec, errors.New("cluster is required")
	}
	return spec, nil
}

type topic struct {
	Name        string `yaml:"name"`
	Partitions  int    `yaml:"partitions"`
	Replication int    `yaml:"replication"`
}

type acl struct {
	Principal string   `yaml:"principal"`
	Resource  string   `yaml:"resource"`
	Ops       []string `yaml:"ops"`
}

type connector struct {
	Name        string `yaml:"name"`
	Class       string `yaml:"class"`
	Database    string `yaml:"database"`
	Table       string `yaml:"table"`
	TopicPrefix string `yaml:"topic_prefix"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

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
	tfvars, err := planApply(spec, defaultMinISR)
	if err != nil {
		return err
	}
	root := filepath.Dir(filepath.Dir(tfDir))
	stateDir := projectStateDir(root, spec.Project)
	tfvarsPath, statePath, err := writeApplyFiles(stateDir, tfvars)
	if err != nil {
		return err
	}

	if err := runTerraform(tfDir, "init"); err != nil {
		return err
	}
	return runTerraform(tfDir, "apply", "-auto-approve", "-state="+statePath, "-var-file="+tfvarsPath)
}

const defaultMinISR = 2

func planApply(spec projectFile, minISR int) (string, error) {
	if len(spec.Topics) != 1 {
		return "", errors.New("exactly one topic is required")
	}
	if len(spec.ACLs) != 1 {
		return "", errors.New("exactly one acl is required")
	}
	if len(spec.Connectors) != 1 {
		return "", errors.New("exactly one connector is required")
	}
	t := spec.Topics[0]
	a := spec.ACLs[0]
	c := spec.Connectors[0]
	if t.Name == "" || t.Partitions < 1 || t.Replication < 1 {
		return "", errors.New("topic needs name, partitions, replication")
	}
	if t.Replication < minISR {
		return "", fmt.Errorf("replication %d is below min-ISR %d", t.Replication, minISR)
	}
	if a.Principal == "" || a.Resource == "" {
		return "", errors.New("acl needs principal and resource")
	}
	if len(a.Ops) == 0 {
		return "", errors.New("acl needs ops")
	}
	if c.Name == "" || c.Class == "" || c.Database == "" || c.Table == "" || c.TopicPrefix == "" {
		return "", errors.New("connector needs name, class, database, table, topic_prefix")
	}

	prefix := spec.Prefix
	if prefix == "" {
		prefix = spec.Project
	}
	topicName := t.Name
	dotted := prefix + "."
	if !strings.HasPrefix(topicName, dotted) {
		topicName = dotted + topicName
	}

	var b strings.Builder
	writeStr(&b, "topic_name", topicName)
	writeNum(&b, "partitions", t.Partitions)
	writeNum(&b, "replicas", t.Replication)
	writeNum(&b, "min_insync_replicas", minISR)
	writeStr(&b, "principal", a.Principal)
	writeStr(&b, "acl_resource", a.Resource)
	writeList(&b, "topic_ops", a.Ops)
	writeStr(&b, "connector_name", c.Name)
	writeStr(&b, "connector_class", c.Class)
	writeStr(&b, "connector_database", c.Database)
	writeStr(&b, "connector_table", c.Table)
	writeStr(&b, "connector_topic_prefix", c.TopicPrefix)
	return b.String(), nil
}

func projectStateDir(root, project string) string {
	return filepath.Join(root, ".ingestctl", project)
}

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

func writeStr(b *strings.Builder, key, v string) {
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.Quote(v))
	b.WriteByte('\n')
}

func writeNum(b *strings.Builder, key string, v int) {
	b.WriteString(key)
	b.WriteString(" = ")
	b.WriteString(strconv.Itoa(v))
	b.WriteByte('\n')
}

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

func readYAML(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		return raw, nil
	}
	tfDir, ferr := findTFDir()
	if ferr != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(tfDir)), path))
}

func findTFDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, "kind", "tf")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("kind/tf not found; run from the repo")
		}
		dir = parent
	}
}

func runTerraform(dir string, args ...string) error {
	cmd := exec.Command("terraform", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

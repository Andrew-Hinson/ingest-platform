// ingestctl apply -f tenants/acme.yaml
// Parses tenant YAML, writes kind/tf/terraform.tfvars, runs terraform init/apply.
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

type tenantFile struct {
	Tenant string  `yaml:"tenant"`
	Topics []topic `yaml:"topics"`
	ACLs   []acl   `yaml:"acls"`
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

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "apply" {
		return errors.New("usage: ingestctl apply -f <tenant.yaml>")
	}

	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("f", "", "tenant YAML")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("usage: ingestctl apply -f <tenant.yaml>")
	}

	raw, err := readTenant(*file)
	if err != nil {
		return err
	}
	var spec tenantFile
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return err
	}
	tfDir, err := findTFDir()
	if err != nil {
		return err
	}
	tfvars, err := planApply(spec, readLastTenant(tfDir), defaultMinISR)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tfDir, "terraform.tfvars"), []byte(tfvars), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tfDir, lastTenantFile), []byte(spec.Tenant+"\n"), 0644); err != nil {
		return err
	}

	if err := runTerraform(tfDir, "init"); err != nil {
		return err
	}
	return runTerraform(tfDir, "apply", "-auto-approve")
}

const (
	defaultMinISR  = 2
	lastTenantFile = ".ingestctl-tenant"
)

func planApply(spec tenantFile, lastTenant string, minISR int) (string, error) {
	if spec.Tenant == "" {
		return "", errors.New("tenant is required")
	}
	if lastTenant != "" && lastTenant != spec.Tenant {
		return "", fmt.Errorf("tenant %s would clobber %s", spec.Tenant, lastTenant)
	}
	if len(spec.Topics) != 1 {
		return "", errors.New("exactly one topic is required")
	}
	if len(spec.ACLs) != 1 {
		return "", errors.New("exactly one acl is required")
	}
	t := spec.Topics[0]
	a := spec.ACLs[0]
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

	var b strings.Builder
	writeStr(&b, "tenant", spec.Tenant)
	writeStr(&b, "topic_name", t.Name)
	writeNum(&b, "partitions", t.Partitions)
	writeNum(&b, "replicas", t.Replication)
	writeNum(&b, "min_insync_replicas", minISR)
	writeStr(&b, "principal", a.Principal)
	writeStr(&b, "prefix", a.Resource)
	writeList(&b, "topic_ops", a.Ops)
	return b.String(), nil
}

func readLastTenant(tfDir string) string {
	raw, err := os.ReadFile(filepath.Join(tfDir, lastTenantFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
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

func readTenant(path string) ([]byte, error) {
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

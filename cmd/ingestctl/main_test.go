package main

import (
	"strings"
	"testing"
)

func validSpec() tenantFile {
	return tenantFile{
		Tenant: "acme",
		Topics: []topic{{Name: "acme.orders", Partitions: 6, Replication: 3}},
		ACLs:   []acl{{Principal: "acme", Resource: "acme.", Ops: []string{"Read", "Write", "Describe"}}},
	}
}

func TestPlanApply_opsPassThrough(t *testing.T) {
	spec := validSpec()
	spec.ACLs[0].Ops = []string{"Read"}
	out, err := planApply(spec, "acme", 2)
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

func TestPlanApply_tenantMismatch(t *testing.T) {
	_, err := planApply(validSpec(), "bravo", 2)
	if err == nil {
		t.Fatal("expected error when applying acme over bravo")
	}
}

func TestPlanApply_sameTenant(t *testing.T) {
	if _, err := planApply(validSpec(), "acme", 2); err != nil {
		t.Fatal(err)
	}
}

func TestPlanApply_rfBelowMinISR(t *testing.T) {
	spec := validSpec()
	spec.Topics[0].Replication = 1
	if _, err := planApply(spec, "", 2); err == nil {
		t.Fatal("expected error when replication is below min-ISR")
	}
}

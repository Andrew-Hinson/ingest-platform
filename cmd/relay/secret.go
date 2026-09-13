package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type instanceCreds struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
}

func retrieveInstanceSecret(name string) (instanceCreds, error) {
	out, err := exec.Command("aws", "secretsmanager", "get-secret-value", "--secret-id", name, "--query", "SecretString", "--output", "text").Output()
	if err != nil {
		return instanceCreds{}, fmt.Errorf("instance secret %s: %w", name, err)
	}
	var creds instanceCreds
	if err := json.Unmarshal(out, &creds); err != nil {
		return instanceCreds{}, err
	}
	if creds.User == "" || creds.Password == "" {
		return instanceCreds{}, fmt.Errorf("instance secret %s missing user or password", name)
	}
	return creds, nil
}

func resolveEndpoint(creds instanceCreds, instance string, create bool) (string, error) {
	if creds.Host != "" {
		return creds.Host, nil
	}
	if create {
		return instance, nil
	}
	out, err := exec.Command(
		"aws", "rds", "describe-db-instances",
		"--db-instance-identifier", instance,
		"--query", "DBInstances[0].Endpoint.Address",
		"--output", "text",
	).Output()
	if err != nil {
		return "", fmt.Errorf("instance endpoint %s: %w", instance, err)
	}
	host := strings.TrimSpace(string(out))
	if host == "" || host == "None" {
		return "", fmt.Errorf("instance endpoint %s is empty", instance)
	}
	return host, nil
}

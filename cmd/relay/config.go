package main

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

type configFile struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Name       string       `yaml:"name"`
	Cluster    string       `yaml:"cluster"`
	Prefix     string       `yaml:"prefix"`
	Instance   instanceSpec `yaml:"instance"`
	Database   databaseSpec `yaml:"database"`
	Tables     []table      `yaml:"tables"`
	Kafka      kafkaSpec    `yaml:"kafka"`
}

type instanceSpec struct {
	Create bool   `yaml:"create"`
	Name   string `yaml:"name"`
}

type databaseSpec struct {
	Name string `yaml:"name"`
}

type table struct {
	Name    string   `yaml:"name"`
	Schema  string   `yaml:"schema"`
	Columns []column `yaml:"columns"`
}

type column struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Nullable   *bool  `yaml:"nullable"`
	PrimaryKey bool   `yaml:"primary_key"`
}

type kafkaSpec struct {
	Partitions        *int `yaml:"partitions"`
	Replicas          *int `yaml:"replicas"`
	MinInsyncReplicas *int `yaml:"min.insync.replicas"`
}

func parseConfig(raw []byte) (configFile, error) {
	var spec configFile
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil {
		return spec, err
	}
	if spec.APIVersion != "relay/v1" {
		return spec, errors.New("apiVersion must be relay/v1")
	}
	if spec.Kind != "Config" {
		return spec, errors.New("kind must be Config")
	}
	if spec.Name == "" {
		return spec, errors.New("name is required")
	}
	if spec.Cluster == "" {
		return spec, errors.New("cluster is required")
	}
	if !spec.Instance.Create && spec.Instance.Name == "" {
		return spec, errors.New("attach instance requires name")
	}
	instName := instanceName(spec)
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

func instanceName(spec configFile) string {
	if spec.Instance.Name != "" {
		return spec.Instance.Name
	}
	return spec.Name
}

func databaseName(spec configFile) string {
	if spec.Database.Name != "" {
		return spec.Database.Name
	}
	return spec.Name
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

package main

import (
	"fmt"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

const customConf = `
---
HelpText: "Please refer to https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING#L632"
PatchScopes:
  HAProxy Standard Scope:
    - MINOR
    - MEDIUM
    - MAJOR
    - CRITICAL
PatchTypes:
  SPECIAL patch:
    Values:
      - SPEC
  HAProxy Standard Patch:
    Values:
      - BUG
      - BUILD
      - CLEANUP
      - DOC
      - LICENSE
      - OPTIM
      - RELEASE
      - REORG
      - TEST
      - REVERT
    Scope: HAProxy Standard Scope
  HAProxy Standard Feature Commit:
    Values:
      - MINOR
      - MEDIUM
      - MAJOR
      - CRITICAL
TagOrder:
  - PatchTypes:
      - SPECIAL patch
    Optional: true
  - PatchTypes:
      - HAProxy Standard Patch
      - HAProxy Standard Feature Commit
`

func LoadCommitPolicyData(config string) (CommitPolicyConfig, error) {
	var commitPolicy CommitPolicyConfig

	if err := yaml.Unmarshal([]byte(config), &commitPolicy); err != nil {
		return CommitPolicyConfig{}, fmt.Errorf("error loading commit policy: %w", err)
	}

	return commitPolicy, nil
}

func TestDifferentPolicy(t *testing.T) {
	t.Parallel()

	c, _ := LoadCommitPolicyData(customConf)

	testsSpec := []struct {
		name    string
		subject string
		wantErr bool
	}{
		{
			name:    "valid type and severity",
			subject: "SPEC: BUG/MEDIUM: config: add default location of path to the configuration file",
			wantErr: false,
		},
		{
			name:    "invalid type RANDOM",
			subject: "RANDOM: BUG/MEDIUM: config: add default location of path to the configuration file",
			wantErr: true,
		},
		{
			name:    "invalid type",
			subject: "SPEC: HEHEEEEEE/MEDIUM: config: add default location of path to the configuration file",
			wantErr: true,
		},
		{
			name:    "invalid severity",
			subject: "SPEC: BUG/HEHEEEEEE: config: add default location of path to the configuration file",
			wantErr: true,
		},
		{
			name:    "no existant aditional type",
			subject: "SPEC: BUG/MINOR: CI: config: add default location of path to the configuration file",
			wantErr: true,
		},
	}
	testsSpec = append(testsSpec, tests...)

	for _, tt := range testsSpec {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := c.CheckSubject([]byte(tt.subject)); (err != nil) != tt.wantErr {
				t.Errorf("checkSubject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

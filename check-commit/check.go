package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode/utf8"

	yaml "gopkg.in/yaml.v2"
)

type patchTypeT struct {
	Values []string `yaml:"Values"`
	Scope  string   `yaml:"Scope"`
}

type tagAlternativesT struct {
	PatchTypes []string `yaml:"PatchTypes"`
	Optional   bool     `yaml:"Optional"`
}

type CommitPolicyConfig struct {
	PatchScopes map[string][]string   `yaml:"PatchScopes"`
	PatchTypes  map[string]patchTypeT `yaml:"PatchTypes"`
	TagOrder    []tagAlternativesT    `yaml:"TagOrder"`
	HelpText    string                `yaml:"HelpText"`
}

const (
	defaultConf = `
---
HelpText: "Please refer to https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING#L632"
PatchScopes:
  HAProxy Standard Scope:
    - MINOR
    - MEDIUM
    - MAJOR
    - CRITICAL
PatchTypes:
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
    - HAProxy Standard Patch
    - HAProxy Standard Feature Commit
`

	minSubjectParts = 3
	maxSubjectParts = 15
	minSubjectLen   = 15
	maxSubjectLen   = 100
)

var ErrSubjectMessageFormat = errors.New("invalid subject message format")

func checkSubjectText(subject string) error {
	subjectLen := utf8.RuneCountInString(subject)
	subjectParts := strings.Fields(subject)
	subjectPartsLen := len(subjectParts)

	if subject != strings.Join(subjectParts, " ") {
		log.Printf("malformatted subject string (trailing or double spaces?): '%s'\n", subject)
	}

	if subjectPartsLen < minSubjectParts || subjectPartsLen > maxSubjectParts {
		return fmt.Errorf(
			"subject word count out of bounds [words %d < %d < %d] '%s': %w",
			minSubjectParts, subjectPartsLen, maxSubjectParts, subjectParts, ErrSubjectMessageFormat)
	}

	if subjectLen < minSubjectLen || subjectLen > maxSubjectLen {
		return fmt.Errorf(
			"subject length out of bounds [len %d < %d < %d] '%s': %w",
			minSubjectLen, subjectLen, maxSubjectLen, subject, ErrSubjectMessageFormat)
	}

	return nil
}

func (c CommitPolicyConfig) CheckPatchTypes(tag, severity string, patchTypeName string) bool {
	tagScopeOK := false

	for _, allowedTag := range c.PatchTypes[patchTypeName].Values {
		if tag == allowedTag {
			if severity == "" {
				tagScopeOK = true

				break
			}

			if c.PatchTypes[patchTypeName].Scope == "" {
				log.Printf("unable to verify severity %s without definitions", severity)

				break // subject has severity but there is no definition to verify it
			}

			for _, allowedScope := range c.PatchScopes[c.PatchTypes[patchTypeName].Scope] {
				if severity == allowedScope {
					tagScopeOK = true

					break
				}
			}
		}
	}

	return tagScopeOK
}

var ErrTagScope = errors.New("invalid tag and or severity")

func (c CommitPolicyConfig) CheckSubject(rawSubject []byte) error {
	// 5 subgroups, 4. is "/severity", 5. is "severity"
	r := regexp.MustCompile(`^(?P<match>(?P<tag>[A-Z]+)(\/(?P<severity>[A-Z]+))?: )`)

	tTag := []byte("$tag")
	tScope := []byte("$severity")
	result := []byte{}

	var tag, severity string

	for _, tagAlternative := range c.TagOrder {

		tagOK := tagAlternative.Optional
		for _, pType := range tagAlternative.PatchTypes { // we allow more than one set of tags in a position

			submatch := r.FindSubmatchIndex(rawSubject)
			if len(submatch) == 0 { // no match
				continue
			}

			tagPart := rawSubject[submatch[0]:submatch[1]]

			tag = string(r.Expand(result, tTag, tagPart, submatch))
			severity = string(r.Expand(result, tScope, tagPart, submatch))

			if c.CheckPatchTypes(tag, severity, pType) { // we found what we were looking for, so consume input
				rawSubject = rawSubject[submatch[1]:]
				tagOK = tagOK || true
			}
		}

		if !tagOK {
			return fmt.Errorf("invalid tag or no tag found: %w", ErrTagScope)
		}
	}

	return checkSubjectText(string(rawSubject))
}

func (c CommitPolicyConfig) IsEmpty() bool {
	c1, _ := yaml.Marshal(c)
	c2, _ := yaml.Marshal(new(CommitPolicyConfig)) // empty config

	return string(c1) == string(c2)
}

type gitEnv struct {
	Ref  string
	Base string
}

type gitEnvVars struct {
	EnvName string
	RefVar  string
	BaseVar string
}

var ErrGitEnvironment = errors.New("git environment error")

func readGitEnvironment() (*gitEnv, error) {
	knownVars := []gitEnvVars{
		{"Github", "GITHUB_REF", "GITHUB_BASE_REF"},
		{"Gitlab", "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"},
	}

	var ref, base string
	for _, vars := range knownVars {
		ref = os.Getenv(vars.RefVar)
		base = os.Getenv(vars.BaseVar)

		if ref != "" && base != "" {
			log.Printf("detected %s environment\n", vars.EnvName)

			return &gitEnv{
				Ref:  ref,
				Base: base,
			}, nil
		}
	}

	return nil, fmt.Errorf("no suitable git environment variables found %w", ErrGitEnvironment)
}

func LoadCommitPolicy(filename string) (CommitPolicyConfig, error) {
	var commitPolicy CommitPolicyConfig

	var config string

	if data, err := ioutil.ReadFile(filename); err != nil {
		log.Printf("error reading config (%s), using built-in fallback configuration (HAProxy defaults)", err)

		config = defaultConf
	} else {
		config = string(data)
	}

	if err := yaml.Unmarshal([]byte(config), &commitPolicy); err != nil {
		return CommitPolicyConfig{}, fmt.Errorf("error loading commit policy: %w", err)
	}

	return commitPolicy, nil
}

func main() {
	commitPolicy, err := LoadCommitPolicy(".check-commit.yml")
	if err != nil {
		log.Fatalf("error reading configuration: %s", err)
	}

	if commitPolicy.IsEmpty() {
		log.Printf("WARNING: using empty configuration (i.e. no verification)")
	}

	gitEnv, err := readGitEnvironment()
	if err != nil {
		log.Fatalf("couldn't auto-detect running environment, please set GITHUB_REF and GITHUB_BASE_REF manually")
	}

	commitRange := fmt.Sprintf("%s...%s", gitEnv.Base, gitEnv.Ref)

	out, err := exec.Command("git", "log", commitRange, "--pretty=format:'%s'").Output()
	if err != nil {
		log.Fatalf("Unable to get log subject '%s'", err)
	}

	// Check subject
	errors := false

	for _, subject := range bytes.Split(out, []byte("\n")) {

		subject = bytes.Trim(subject, "'")
		if err := commitPolicy.CheckSubject(subject); err != nil {
			log.Printf("%s, original subject message '%s'", err, string(subject))

			errors = true
		}
	}

	if errors {
		log.Printf("encountered one or more commit message errors\n")
		log.Fatalf("%s\n", commitPolicy.HelpText)
	} else {
		log.Printf("check completed without errors\n")
	}
}

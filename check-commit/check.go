package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

	MINSUBJECTPARTS = 3
	MAXSUBJECTPARTS = 15
	MINSUBJECTLEN   = 15
	MAXSUBJECTLEN   = 100

	GITHUB = "Github"
)

var ErrSubjectMessageFormat = errors.New("invalid subject message format")

func checkSubjectText(subject string) error {
	subjectLen := utf8.RuneCountInString(subject)
	subjectParts := strings.Fields(subject)
	subjectPartsLen := len(subjectParts)

	if subject != strings.Join(subjectParts, " ") {
		return fmt.Errorf(
			"malformatted subject string (trailing or double spaces?): '%s' (%w)",
			subject, ErrSubjectMessageFormat)
	}

	if subjectPartsLen < MINSUBJECTPARTS || subjectPartsLen > MAXSUBJECTPARTS {
		return fmt.Errorf(
			"subject word count out of bounds [words %d < %d < %d] '%s': %w",
			MINSUBJECTPARTS, subjectPartsLen, MAXSUBJECTPARTS, subjectParts, ErrSubjectMessageFormat)
	}

	if subjectLen < MINSUBJECTLEN || subjectLen > MAXSUBJECTLEN {
		return fmt.Errorf(
			"subject length out of bounds [len %d < %d < %d] '%s': %w",
			MINSUBJECTLEN, subjectLen, MAXSUBJECTLEN, subject, ErrSubjectMessageFormat)
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
	// check for ascii-only before anything else
	for i := 0; i < len(rawSubject); i++ {
		if rawSubject[i] > unicode.MaxASCII {
			log.Printf("non-ascii characters detected in in subject:\n%s", hex.Dump(rawSubject))

			return fmt.Errorf("non-ascii characters in commit subject: %w", ErrTagScope)
		}
	}
	// 5 subgroups, 4. is "/severity", 5. is "severity"
	r := regexp.MustCompile(`^(?P<match>(?P<tag>[A-Z]+)(\/(?P<severity>[A-Z]+))?: )`)

	tTag := []byte("$tag")
	tScope := []byte("$severity")
	result := []byte{}

	candidates := []string{}

	var tag, severity string

	for _, tagAlternative := range c.TagOrder {
		tagOK := tagAlternative.Optional

		submatch := r.FindSubmatchIndex(rawSubject)
		if len(submatch) == 0 { // no match
			continue
		}

		tagPart := rawSubject[submatch[0]:submatch[1]]

		tag = string(r.Expand(result, tTag, tagPart, submatch))
		severity = string(r.Expand(result, tScope, tagPart, submatch))

		for _, pType := range tagAlternative.PatchTypes { // we allow more than one set of tags in a position
			if c.CheckPatchTypes(tag, severity, pType) { // we found what we were looking for, so consume input
				rawSubject = rawSubject[submatch[1]:]
				tagOK = tagOK || true

				break
			}
		}

		candidates = append(candidates, string(tagPart))

		if !tagOK {
			log.Printf("unable to find match in %s\n", candidates)

			return fmt.Errorf("invalid tag or no tag found, searched through [%s]: %w",
				strings.Join(tagAlternative.PatchTypes, ", "), ErrTagScope)
		}
	}

	submatch := r.FindSubmatchIndex(rawSubject)
	if len(submatch) != 0 { // no match
		return fmt.Errorf("detected unprocessed tags, %w", ErrTagScope)
	}

	return checkSubjectText(string(rawSubject))
}

func (c CommitPolicyConfig) IsEmpty() bool {
	c1, _ := yaml.Marshal(c)
	c2, _ := yaml.Marshal(new(CommitPolicyConfig)) // empty config

	return string(c1) == string(c2)
}

type gitEnv struct {
	EnvName string
	Event   string
	Ref     string
	Base    string
}

type gitEnvVars struct {
	EnvName  string
	EventVar string
	RefVar   string
	BaseVar  string
}

var ErrGitEnvironment = errors.New("git environment error")

func readGitEnvironment() (*gitEnv, error) {
	knownVars := []gitEnvVars{
		{GITHUB, "GITHUB_EVENT_NAME", "GITHUB_SHA", "GITHUB_BASE_REF"},
		{"Gitlab", "CI_PIPELINE_SOURCE", "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"},
		{"Gitlab-commit", "CI_PIPELINE_SOURCE", "CI_COMMIT_SHA", "CI_DEFAULT_BRANCH"},
	}

	var ref, base string

	for _, vars := range knownVars {
		event := os.Getenv(vars.EventVar)
		ref = os.Getenv(vars.RefVar)
		base = os.Getenv(vars.BaseVar)

		if !(ref == "" && base == "") || (vars.EnvName == GITHUB && event == "push") {
			log.Printf("detected %s environment\n", vars.EnvName)
			log.Printf("using event '%s' with refs '%s' and '%s'\n", event, ref, base)

			return &gitEnv{
				EnvName: vars.EnvName,
				Event:   event,
				Ref:     ref,
				Base:    base,
			}, nil
		}
	}

	return nil, fmt.Errorf("no suitable git environment variables found: %w", ErrGitEnvironment)
}

func LoadCommitPolicy(filename string) (CommitPolicyConfig, error) {
	var commitPolicy CommitPolicyConfig

	var config string

	if data, err := ioutil.ReadFile(filename); err != nil {
		log.Printf("warning: using built-in fallback configuration with HAProxy defaults (%s)", err)

		config = defaultConf
	} else {
		config = string(data)
	}

	if err := yaml.Unmarshal([]byte(config), &commitPolicy); err != nil {
		return CommitPolicyConfig{}, fmt.Errorf("error loading commit policy: %w", err)
	}

	return commitPolicy, nil
}

func hashesFromRefs(repo *git.Repository, repoEnv *gitEnv) ([]*plumbing.Hash, []*object.Commit) {
	var refStrings []string
	refStrings = append(refStrings, repoEnv.Ref)

	if !(repoEnv.EnvName == GITHUB && repoEnv.Event == "push") { // for Github push we only have the last commit
		refStrings = append(refStrings, fmt.Sprintf("refs/remotes/origin/%s", repoEnv.Base))
	}

	hashes := make([]*plumbing.Hash, 0, 2)

	for _, refString := range refStrings {
		hash, err := repo.ResolveRevision(plumbing.Revision(refString))
		if err != nil {
			log.Fatalf("unable to resolve revision %s to hash: %s", refString, err)
		}

		hashes = append(hashes, hash)
	}

	commits := make([]*object.Commit, 0, 2)

	for _, hash := range hashes {
		commit, err := repo.CommitObject(*hash)
		if err != nil {
			log.Fatalf("unable to find commit %s", hash.String())
		}

		commits = append(commits, commit)
	}

	return hashes, commits
}

var ErrReachedMergeBase = errors.New("reached Merge Base")

func getCommitSubjects(repo *git.Repository, repoEnv *gitEnv) ([]string, error) {
	hashes, commits := hashesFromRefs(repo, repoEnv)

	if len(commits) == 1 { // just the last commit
		return []string{strings.Split(commits[0].Message, "\n")[0]}, nil
	}

	mergeBase, err := commits[0].MergeBase(commits[1])
	if err != nil {
		log.Fatalf("repo history error %s", err)
	}

	logOptions := new(git.LogOptions)
	logOptions.From = *hashes[0]
	logOptions.Order = git.LogOrderCommitterTime

	cIter, err := repo.Log(logOptions)
	if err != nil {
		log.Fatalf("error getting commit log %s", err)
	}

	var subjects []string

	gitlabMergeRegex := regexp.MustCompile(`Merge \w{40} into \w{40}`)

	err = cIter.ForEach(func(c *object.Commit) error {
		if c.Hash == mergeBase[0].Hash {
			return ErrReachedMergeBase
		}
		subjectOnly := strings.Split(c.Message, "\n")[0]

		if !(repoEnv.EnvName == GITHUB && repoEnv.Event == "pull_request" && gitlabMergeRegex.Match([]byte(c.Message))) {
			// ignore github pull request commits with subject "Merge x into y", these get added automatically by github
			subjects = append(subjects, subjectOnly)
			log.Printf("collected commit hash %s, subject '%s'", c.Hash, subjectOnly)
		} else {
			log.Printf("ignoring a pull_request Merge commit hash, %s subject '%s'", c.Hash, subjectOnly)
		}

		return nil
	})
	if !errors.Is(err, ErrReachedMergeBase) {
		return []string{}, fmt.Errorf("error tracing commit history: %w", err)
	}

	return subjects, nil
}

var ErrSubjectList = errors.New("subjects contain errors")

func (c CommitPolicyConfig) CheckSubjectList(subjects []string) error {
	errors := false

	for _, subject := range subjects {
		subject = strings.Trim(subject, "'")
		if err := c.CheckSubject([]byte(subject)); err != nil {
			log.Printf("%s, original subject message '%s'", err, subject)

			errors = true
		}
	}

	if errors {
		return ErrSubjectList
	}

	return nil
}

const requiredCmdlineArgs = 2

func main() {
	var repoPath string

	if len(os.Args) < requiredCmdlineArgs {
		repoPath = "."
	} else {
		repoPath = os.Args[1]
	}

	commitPolicy, err := LoadCommitPolicy(path.Join(repoPath, ".check-commit.yml"))
	if err != nil {
		log.Fatalf("error reading configuration: %s", err)
	}

	if commitPolicy.IsEmpty() {
		log.Printf("WARNING: using empty configuration (i.e. no verification)")
	}

	gitEnv, err := readGitEnvironment()
	if err != nil {
		log.Fatalf("couldn't auto-detect running environment, please set GITHUB_REF and GITHUB_BASE_REF manually: %s", err)
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		log.Fatalf("couldn't open git local git repo: %s", err)
	}

	subjects, err := getCommitSubjects(repo, gitEnv)
	if err != nil {
		log.Fatalf("error getting commit subjects: %s", err)
	}

	if err := commitPolicy.CheckSubjectList(subjects); err != nil {
		log.Printf("encountered one or more commit message errors\n")
		log.Fatalf("%s\n", commitPolicy.HelpText)
	}

	log.Printf("check completed without errors\n")
}

package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/go-github/v35/github"

	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
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
	GITLAB = "Gitlab"
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
	EnvName     string
	URL         string
	Token       string
	ProjectID   string
	PMRequestID string
}

type gitEnvVars struct {
	EnvName   string
	ApiUrl    string
	ApiToken  string
	ProjectID string
	RequestID string
}

var ErrGitEnvironment = errors.New("git environment error")

func readGitEnvironment() (*gitEnv, error) {
	knownVars := []gitEnvVars{
		{GITHUB, "GITHUB_API_URL", "API_TOKEN", "GITHUB_REPOSITORY", "GITHUB_SHA"},
		{GITLAB, "CI_API_V4_URL", "CI_JOB_TOKEN", "CI_MERGE_REQUEST_PROJECT_ID", "CI_MERGE_REQUEST_ID"},
	}

	for _, vars := range knownVars {
		url := os.Getenv(vars.ApiUrl)
		token := os.Getenv(vars.ApiToken)
		project := os.Getenv(vars.ProjectID)
		request := os.Getenv(vars.RequestID)

		if !(url == "" && token == "" && project == "" && request == "") {
			log.Printf("detected %s environment\n", vars.EnvName)
			log.Printf("using api url '%s'\n", url)

			return &gitEnv{
				EnvName:     vars.EnvName,
				URL:         url,
				Token:       token,
				ProjectID:   project,
				PMRequestID: request,
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

func getGithubCommitSubjects(token string, repo string, sha string) ([]string, error) {
	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	githubClient := github.NewClient(tc)

	repoSlice := strings.SplitN(repo, "/", 2)

	prs, _, err := githubClient.PullRequests.ListPullRequestsWithCommit(ctx, repoSlice[0], repoSlice[1], sha, &github.PullRequestListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error fetching prs for commit %s: %w", sha, err)
	}

	subjects := []string{}
	if len(prs) > 0 {
		// Check the latest PR with this commit
		prNo := prs[0].GetNumber()
		commits, _, err := githubClient.PullRequests.ListCommits(ctx, repoSlice[0], repoSlice[1], prNo, &github.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("error fetching commits: %w", err)
		}
		for _, c := range commits {
			l := strings.SplitN(c.Commit.GetMessage(), "\n", 2)
			if len(l) > 0 {
				subjects = append(subjects, l[0])
			}
		}
	} else {
		// no PRs, event was a direct push, check only latest commit
		c, _, err := githubClient.Repositories.GetCommit(ctx, repoSlice[0], repoSlice[1], sha)
		if err != nil {
			return nil, fmt.Errorf("error fetching commit %s: %w", sha, err)
		}
		l := strings.SplitN(c.Commit.GetMessage(), "\n", 2)
		if len(l) > 0 {
			subjects = append(subjects, l[0])
		}
	}

	return subjects, nil
}

func gitGitlabCommitSubjects(url string, token string, project string, mr string) ([]string, error) {
	gitlabClient, err := gitlab.NewClient(token, gitlab.WithBaseURL(url))
	if err != nil {
		log.Fatalf("Failed to create gitlab client: %v", err)
	}

	mrID, err := strconv.Atoi(mr)
	if err != nil {
		return nil, fmt.Errorf("invalid merge request id %s", mr)
	}
	commits, _, err := gitlabClient.MergeRequests.GetMergeRequestCommits(project, mrID, &gitlab.GetMergeRequestCommitsOptions{})
	if err != nil {
		return nil, fmt.Errorf("error fetching commits: %w", err)
	}

	subjects := []string{}
	for _, c := range commits {
		l := strings.SplitN(c.Message, "\n", 2)
		if len(l) > 0 {
			subjects = append(subjects, l[0])
		}
	}

	return subjects, nil
}

func getCommitSubjects(repoEnv *gitEnv) ([]string, error) {
	if repoEnv.EnvName == GITHUB {
		return getGithubCommitSubjects(repoEnv.Token, repoEnv.ProjectID, repoEnv.PMRequestID)
	} else if repoEnv.EnvName == GITLAB {
		return gitGitlabCommitSubjects(repoEnv.URL, repoEnv.Token, repoEnv.ProjectID, repoEnv.PMRequestID)
	}
	return nil, fmt.Errorf("unrecognized git environment %s", repoEnv.EnvName)
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

	subjects, err := getCommitSubjects(gitEnv)
	if err != nil {
		log.Fatalf("error getting commit subjects: %s", err)
	}

	if err := commitPolicy.CheckSubjectList(subjects); err != nil {
		log.Printf("encountered one or more commit message errors\n")
		log.Fatalf("%s\n", commitPolicy.HelpText)
	}

	log.Printf("check completed without errors\n")
}

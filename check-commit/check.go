package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type (
	PatchType     string
	PatchSeverity string
)

const (
	BUG     PatchType = "BUG"
	BUILD   PatchType = "BUILD"
	CLEANUP PatchType = "CLEANUP"
	DOC     PatchType = "DOC"
	LICENSE PatchType = "LICENSE"
	OPTIM   PatchType = "OPTIM"
	RELEASE PatchType = "RELEASE"
	REORG   PatchType = "REORG"
	TEST    PatchType = "TEST"
	REVERT  PatchType = "REVERT"
)

const (
	MINOR    PatchSeverity = "MINOR"
	MEDIUM   PatchSeverity = "MEDIUM"
	MAJOR    PatchSeverity = "MAJOR"
	CRITICAL PatchSeverity = "CRITICAL"
)

const guidelinesLink = "Please refer to https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING#L632"

type patchScope_t struct {
	Scope  string
	Values []string
}
type patchType_t struct {
	Values []string
	Scope  string
}
type tagAlternatives_t struct {
	PatchTypes []string
	Optional   bool
}

type prgConfig struct {
	PatchScopes map[string][]string
	PatchTypes  map[string]patchType_t
	TagOrder    []tagAlternatives_t
}

var defaultConf string = `
{
	"PatchScopes": {
		"HAProxy Standard Scope": [
			"MINOR",
			"MEDIUM",
			"MAJOR",
			"CRITICAL"
		]
	},
	"PatchTypes": {
		"HAProxy Standard Patch": {
			"Values": [
				"BUG",
				"BUILD",
				"CLEANUP",
				"DOC",
				"LICENSE",
				"OPTIM",
				"RELEASE",
				"REORG",
				"TEST",
				"REVERT"
			],
			"Scope": "HAProxy Standard Scope"
		},
		"HAPEE Commit": {
			"Values": [
				"EE"
			]
		},
		"HAProxy Standard Feature Commit": {
			"Values": [
				"MINOR",
				"MEDIUM",
				"MAJOR",
				"CRITICAL"
			]
		}
	},
	"TagOrder": [
		{
			"PatchTypes": [
				"HAPEE Commit"
			],
			"Optional": true
		},
		{
			"PatchTypes": [
				"HAProxy Standard Patch",
				"HAProxy Standard Feature Commit"
			]
		}
	]
}
`

var myConfig prgConfig

type parsedConfig struct {
}

func (pt PatchType) IsValid() error {
	switch pt {
	case BUG, BUILD, CLEANUP, DOC, LICENSE, OPTIM, RELEASE, REORG, TEST, REVERT:
		return nil
	}
	return fmt.Errorf("Invalid patch type '%s'", pt)
}

func (ps PatchSeverity) IsValid() error {
	switch ps {
	case MINOR, MEDIUM, MAJOR, CRITICAL:
		return nil
	}
	return fmt.Errorf("Invalid patch severity '%s'", ps)
}

func checkSubject(subject string) error {
	parts := strings.Split(subject, ":")
	if len(parts) < 2 {
		return fmt.Errorf("Incorrect message format '%s'", subject)
	}

	// Commit type
	commitType := strings.Split(parts[0], "/")
	switch len(commitType) {
	case 1:
		errPs := PatchSeverity(commitType[0]).IsValid()
		errPt := PatchType(commitType[0]).IsValid()
		if errPs != nil && errPt != nil {
			return errPs
		}
	case 2:
		if err := PatchType(commitType[0]).IsValid(); err != nil {
			return err
		}
		if err := PatchSeverity(commitType[1]).IsValid(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Incorrect message format '%s'", subject)
	}
	// Commit subject

	subject = strings.Join(parts[1:], " ")
	subjectParts := strings.FieldsFunc(subject, split)

	if len(subjectParts) < 3 {
		return fmt.Errorf("Too short or meaningless commit subject [words %d < 3] '%s'", len(subjectParts), subjectParts)
	}
	if len(subject) < 15 {
		return fmt.Errorf("Too short or meaningless commit subject [len %d < 15]'%s'", len(subject), subject)
	}
	if len(subjectParts) > 15 {
		return fmt.Errorf("Too long commit subject [words %d > 15 - use msg body] '%s'", len(subjectParts), subjectParts)
	}
	if len(subject) > 100 {
		return fmt.Errorf("Too long commit subject [len %d > 100] '%s'", len(subject), subject)
	}
	return nil
}

func checkSubject2(subject string) error {
	rawSubject := []byte(subject)
	r, _ := regexp.Compile("^(?P<match>(?P<tag>[A-Z]+)(\\/(?P<scope>[A-Z]+))?: )") // 5 subgroups, 4. is "/scope", 5. is "scope"

	t_tag := []byte("$tag")
	t_scope := []byte("$scope")
	result := []byte{}

	var tag string
	var scope string

	for _, tagAlternative := range myConfig.TagOrder {
		// log.Printf("processing tagalternative %s\n", tagAlternative)
		tagOK := tagAlternative.Optional
		for _, pType := range tagAlternative.PatchTypes {
			// log.Printf("processing patchtype %s", pType)

			submatch := r.FindSubmatchIndex(rawSubject)
			if len(submatch) == 0 { // no match
				continue
			}
			tagPart := rawSubject[submatch[0]:submatch[1]]

			tag = string(r.Expand(result, t_tag, tagPart, submatch))
			scope = string(r.Expand(result, t_scope, tagPart, submatch))

			tagScopeOK := false

			for _, allowedTag := range myConfig.PatchTypes[pType].Values {
				if tag == allowedTag {
					// log.Printf("found allowed tag %s\n", tag)
					if myConfig.PatchTypes[pType].Scope != "" {
						for _, allowedScope := range myConfig.PatchScopes[myConfig.PatchTypes[pType].Scope] {
							if scope == allowedScope {
								tagScopeOK = true
							}
						}
					} else {
						tagScopeOK = true
					}
				}
			}
			if tagScopeOK { //we found what we were looking for, so consume input
				rawSubject = rawSubject[submatch[1]:]
			}
			tagOK = tagOK || tagScopeOK
			// log.Printf("tag is %s, scope is %s, rest is %s\n", tag, scope, rawSubject)
		}
		if !tagOK {
			return fmt.Errorf("invalid tag or no tag found: %s", tag)
		}
	}

	subjectParts := strings.Fields(subject)

	if subject != strings.Join(subjectParts, " ") {
		log.Printf("malformatted subject string (trailing or double spaces?): '%s'\n", subject)
	}

	if len(subjectParts) < 3 {
		return fmt.Errorf("Too short or meaningless commit subject [words %d < 3] '%s'", len(subjectParts), subjectParts)
	}
	if len(subject) < 15 {
		return fmt.Errorf("Too short or meaningless commit subject [len %d < 15]'%s'", len(subject), subject)
	}
	if len(subjectParts) > 15 {
		return fmt.Errorf("Too long commit subject [words %d > 15 - use msg body] '%s'", len(subjectParts), subjectParts)
	}
	if len(subject) > 100 {
		return fmt.Errorf("Too long commit subject [len %d > 100] '%s'", len(subject), subject)
	}
	return nil
}

func split(r rune) bool {
	return r == ' '
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

var knownVars []gitEnvVars = []gitEnvVars{
	{"Github", "GITHUB_REF", "GITHUB_BASE_REF"},
	{"Gitlab", "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"},
}

func readGitEnvironment() (*gitEnv, error) {
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
	return nil, fmt.Errorf("no suitable git environment variables found")
}

func main() {
	if err := json.Unmarshal([]byte(defaultConf), &myConfig); err != nil {
		log.Fatalf("error reading configuration: %s", err)
	}
	fmt.Printf("%s\n", myConfig)

	var out []byte

	gitEnv, err := readGitEnvironment()
	if err != nil {
		log.Println(err)
		log.Println("falling back to best effort")
		out, err = exec.Command("git", "log", "-1", "--pretty=format:'%s'").Output()
		if err != nil {
			log.Fatal(fmt.Errorf("Unable to get log subject '%s'", err))
		}

		// Handle Merge Request where the subject of last commit has the format:
		// "Merge commitA-ID into commitB-ID"
		// TODO: Make this generic by taking IDs as input params
		subject := strings.Trim((string(out)), "'")
		if strings.HasPrefix(subject, "Merge") {
			log.Println("Handling Merge Request:\n", subject)
			parts := strings.Fields(subject)
			if len(parts) != 4 {
				log.Fatal(fmt.Errorf("Unknown Merge commit format '%s'\n", subject))
			}
			out, err = exec.Command("git", "log", parts[3]+".."+parts[1], "--pretty=format:'%s'").Output()
			if err != nil {
				log.Fatal(fmt.Errorf("Unable to get log subject: '%s'", err))
			}
		}
	} else {
		out, err = exec.Command("git", "log", fmt.Sprintf("%s...%s", gitEnv.Base, gitEnv.Ref), "--pretty=format:'%s'").Output()
		if err != nil {
			log.Fatalf("Unable to get log subject '%s'", err)
		}
	}

	errors := false
	// Check subject
	for _, subject := range strings.Split(string(out), "\n") {
		subject = strings.Trim(subject, "'")
		if err := checkSubject2(string(subject)); err != nil {
			log.Printf("%s, original subject message '%s'", err, subject)
			errors = true
		}
	}
	if errors {
		log.Fatalf("encountered one or more commit message errors\n")
		log.Fatalln(guidelinesLink)
	}
}

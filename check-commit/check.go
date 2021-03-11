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
	HelpText    string
}

var defaultConf string = `
{
	"HelpText": "Please refer to https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING#L632",
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

func checkSubject(subject string) error {
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
					if scope == "" {
						tagScopeOK = true
					} else {
						if myConfig.PatchTypes[pType].Scope == "" {
							log.Printf("subject scope problem")
							break // subject has scope but there is no definition to verify it
						}
						for _, allowedScope := range myConfig.PatchScopes[myConfig.PatchTypes[pType].Scope] {
							if scope == allowedScope {
								tagScopeOK = true
							}
						}
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
			return fmt.Errorf("invalid tag or no tag found: %s/%s", tag, scope)
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
		log.Fatalf("couldn't auto-detect running environment, please set GITHUB_REF and GITHUB_BASE_REF manually")
	}

	out, err = exec.Command("git", "log", fmt.Sprintf("%s...%s", gitEnv.Base, gitEnv.Ref), "--pretty=format:'%s'").Output()
	if err != nil {
		log.Fatalf("Unable to get log subject '%s'", err)
	}

	// Check subject
	errors := false
	for _, subject := range strings.Split(string(out), "\n") {
		subject = strings.Trim(subject, "'")
		if err := checkSubject(string(subject)); err != nil {
			log.Printf("%s, original subject message '%s'", err, subject)
			errors = true
		}
	}

	if errors {
		log.Printf("encountered one or more commit message errors\n")
		log.Fatalf("%s\n", myConfig.HelpText)
	}
}

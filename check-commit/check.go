package main

import (
	"fmt"
	"log"
	"os/exec"
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

func (pt PatchType) IsValid() error {
	switch pt {
	case BUG, BUILD, CLEANUP, DOC, LICENSE, OPTIM, RELEASE, REORG, TEST, REVERT:
		return nil
	}
	return fmt.Errorf("Invalid patch type '%s'\n%s", pt, guidelinesLink)
}

func (ps PatchSeverity) IsValid() error {
	switch ps {
	case MINOR, MEDIUM, MAJOR, CRITICAL:
		return nil
	}
	return fmt.Errorf("Invalid patch severity '%s'\n%s", ps, guidelinesLink)
}

func checkSubject(subject string) error {
	parts := strings.Split(subject, ":")
	if len(parts) < 2 {
		return fmt.Errorf("Incorrect message format '%s'\n%s", subject, guidelinesLink)
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
		return fmt.Errorf("Incorrect message format '%s'\n%s", subject, guidelinesLink)
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

func split(r rune) bool {
	return r == ' '
}

func stripQuotes(input string) string {
	if len(input) > 0 {
		if input[0] == []byte("'")[0] {
			input = input[1:]
		}
		if input[len(input)-1] == []byte("'")[0] {
			input = input[:len(input)-1]
		}
	}
	return input
}

func main() {
	out, err := exec.Command("git", "log", "-1", "--pretty=format:'%s'").Output()
	if err != nil {
		log.Fatal(fmt.Errorf("Unable to get log subject '%s'", err))
	}

	// Handle Merge Request where the subject of last commit has the format:
	// "Merge commitA-ID into commitB-ID"
	// TODO: Make this generic by taking IDs as input params
	subject := stripQuotes(string(out))
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

	// Check subject
	for _, subject = range strings.Split(string(out), "\n") {
		subject = stripQuotes(subject)
		if err := checkSubject(string(subject)); err != nil {
			log.Fatal(err)
		}
	}
}

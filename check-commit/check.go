package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type PatchType string
type PatchSeverity string

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
	case BUG, BUILD, CLEANUP, DOC, LICENSE, OPTIM, RELEASE, REORG, TEST:
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

func main() {
	subject, err := exec.Command("git", "log", "-1", "--pretty=format:'%s'").Output()
	if err != nil {
		log.Fatal(fmt.Errorf("Unable to get log subject %s", err))
	}

	if len(subject) > 0 {
		if subject[0] == []byte("'")[0] {
			subject = subject[1:]
		}
		if subject[len(subject)-1] == []byte("'")[0] {
			subject = subject[:len(subject)-1]
		}
	}
	parts := strings.Split(string(subject), ":")
	if len(parts) < 2 {
		log.Fatal(fmt.Errorf("Incorrect message format\n" + guidelinesLink))
	}

	// Commit type
	commitType := strings.Split(string(parts[0]), "/")
	switch len(commitType) {
	case 1:
		errPs := PatchSeverity(commitType[0]).IsValid()
		errPt := PatchType(commitType[0]).IsValid()
		if errPs != nil && errPt != nil {
			log.Fatal(errPs)
		}
	case 2:
		if err := PatchType(commitType[0]).IsValid(); err != nil {
			log.Fatal(err)
		}
		if err := PatchSeverity(commitType[1]).IsValid(); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal(fmt.Errorf("Incorrect message format\n" + guidelinesLink))
	}
	// Commit subject
	if len(parts[1]) < 20 || len(strings.Split(parts[1], " ")) < 3 {
		log.Fatal(fmt.Errorf("Too short or meaningless commit subject"))
	}
}

![check-commit; golangci-lint](https://github.com/haproxytech/github-actions/actions/workflows/main.yml/badge.svg)

# GitHub Action: Check commit subject is compliant with HAProxy guidelines

This action checks that the commit subject is compliant with the [patch classifying rules](https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING#L632) of HAProxy contribution guidelines. Also it does minimal check for a meaningful message in the commit subject: no less than 20 characters and at least 3 words.

By default only the top commit subject is checked.
If the commit subject has the format: `Merge 'commitA-ID' into 'commitB-ID'`, all commit subjects between 'commitA' and 'commitB' will be checked.

## Examples

### Good

- Bug fix:
```
BUG/MEDIUM: fix set-var parsing bug in config-parser
```
- New minor feature:
```
MINOR: Add path-rewrite annotation
```
- Minor build update:
```
BUILD/MINOR: Add path-rewrite annotation
```

### Bad

- Incorrect patch type
```
bug: fix set-var parsing bug in config-parser
```
- Short commit message 
```
BUG/MEDIUM: fix set-var
```
- Unkown severity
```
BUG/MODERATE: fix set-var parsing bug in config-parser
```


## Inputs

None.

## Usage

```yaml
steps:
  - name: Check out code
    uses: actions/checkout@v2
    with:
      fetch-depth: 0
  - name: check-commit
    uses: docker://haproxytech/check-commit:TAG
```
Here we instruct `checkout@v2` action to fetch the repo history.

Check-commit can work on `pull_request` events by inspecting all commit messages between a merge-base commit of the the target branch (usually main, looked up from `$GITHUB_BASE_REF`) and the tip of the feature branch (`$GITHUB_REF`) branch. If the operation is a `push` only the last commit is checked.

For this to work, the `origin/$GITHUB_BASE_REF` ref has to be a part of the history checked out by `actions/checkout@v2`, so `fetch-depth: 0` is can be used as a safe option to fetch the complete repo history. Setting this to a fixed number will limit how many commits are allowed in a Pull Request to that many commits and will speed up the checkout for large repos.

## Example configuration

If a configuration file (`.check-commit.yml`) is not available in the running directory, a built-in failsafe configuration identical to the one below is used.

```yaml
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
```

### Optional parameters

The program accepts an optional parameter to specify the location (path) of the base of the git repository. This can be useful in certain cases where the checked-out repo is in a non-standard location within the CI environment, compared to the running path from which the check-commit binary is being invoked.

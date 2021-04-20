![check-commit; golangci-lint](https://github.com/haproxytech/github-actions/actions/workflows/main.yml/badge.svg)

# GitHub Action: Check commit subject is compliant with HAProxy guidelines

This action checks that the commit subject is compliant with the [patch classifying rules](https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING#L632) of HAProxy contribution guidelines. Also it does minimal check for a meaningful message in the commit subject: no less than 20 characters and at least 3 words.

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
  - name: check-commit
    uses: docker://haproxytech/check-commit:TAG
    env:
      API_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```
Check-commit works only on `pull_request` events by inspecting all commit messages in a Pull Request. It uses Github API [pull requests API](https://docs.github.com/en/rest/reference/pulls#list-commits-on-a-pull-request) to fetch the commits so API_TOKEN env_variable is required.

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

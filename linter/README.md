# GitHub Action: Go linter for projects

This action uses [golangci-lint](https://github.com/golangci/golangci-lint) to check if code conforms with
the rules that are specified in `.golangci.yml` file hosted in repository

## Inputs

None.

## Usage

```yaml
steps:
  - name: Check out code
    uses: actions/checkout@v2
  - name: check-commit
    uses: docker://haproxytech/linter:latest
```

## Development phase

action can be simulated locally with

```bash
golangci-lint run --enable-all
```

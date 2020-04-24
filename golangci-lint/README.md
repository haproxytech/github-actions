# GitHub Action: Run golangci-lint

This action runs [golangci-lint](https://github.com/golangci/golangci-lint) on HAProxy Golang projects

## Inputs

None.

## Usage

```yaml
steps:
  - name: Check out code
    uses: actions/checkout@v1
  - name: golangci-lint
    uses: haproxytech/github-actions/golangci-lint@master
```

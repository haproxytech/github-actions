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
      fetch-depth: 10
  - name: check-commit
    uses: docker://haproxytech/check-commit:TAG
```
Here we instruct `checkout@v2` action to fetch last 10 commits (by default it fetches only last one) which is required in case of checking multiple commits.

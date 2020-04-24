# ![HAProxy](https://github.com/haproxytech/kubernetes-ingress/raw/master/assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy Github Actions


This repository contains Github Actions used in CI/CD workflows of [HAProxy Technologies](https://www.haproxy.com/) repositories hosted in Github.

## Usage

```yaml
steps:
  - name: Check out code
    uses: actions/checkout@v1
  - name: action-name
    uses: haproxytech/github-actions/action-name@master
```

FROM golang:alpine
LABEL maintainer="mmhedhbi@haproxy.com"
RUN apk add git
COPY check.go /
ENTRYPOINT ["go","run","/check.go"]


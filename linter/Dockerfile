FROM golang:alpine
RUN apk add curl
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.33.0
ENTRYPOINT ["golangci-lint", "run", "--enable-all"]

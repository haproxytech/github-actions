FROM golang:alpine as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go build -o check check.go

FROM alpine:latest
LABEL maintainer="mmhedhbi@haproxy.com"
COPY --from=builder /build/check /check
WORKDIR /
ENTRYPOINT ["/check"]


FROM golang:alpine as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go build -o check check.go

FROM golang:alpine
LABEL maintainer="mmhedhbi@haproxy.com"
RUN apk add git
COPY --from=builder /build/check /check
WORKDIR /
ENTRYPOINT ["/check"]


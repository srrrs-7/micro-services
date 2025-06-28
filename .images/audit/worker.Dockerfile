FROM golang:latest AS builder

COPY ./modules /go/src
WORKDIR /go/src

ARG GOOS=linux
ARG GOARCH=amd64
ARG CGO_ENABLED=false
RUN go build -ldflags="-s -w" -o /go/bin/worker /go/src/audit/src/cmd/worker

FROM alpine:latest
COPY --from=builder /go/bin/worker /usr/local/bin/worker

CMD [ "/usr/local/bin/worker" ]
FROM golang:latest AS builder

COPY ./modules /go/modules
WORKDIR /go/modules/audit/src

ARG GOOS=linux
ARG GOARCH=amd64
ARG CGO_ENABLED=false
RUN go build -ldflags="-s -w" -o /go/bin/worker ./cmd/worker

FROM debian:latest
COPY --from=builder /go/bin/worker /usr/local/bin/worker

CMD [ "/usr/local/bin/worker" ]
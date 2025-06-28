FROM golang:latest AS builder

COPY ./modules /go/src
WORKDIR /go/src

ARG GOOS=linux
ARG GOARCH=amd64
ARG CGO_ENABLED=false
RUN go build -ldflags="-s -w" -o /go/bin/api /go/src/queue/src/api/cmd

FROM alpine:latest
COPY --from=builder /go/bin/api /usr/local/bin/api

EXPOSE 8080

CMD [ "/usr/local/bin/api" ]
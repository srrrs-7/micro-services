FROM golang:1.24-alpine

# protocバイナリ自体をインストール
RUN apk add --no-cache protobuf protobuf-dev fish

# Go用のprotoc pluginsをインストール
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

ENV PATH="/go/bin:${PATH}"
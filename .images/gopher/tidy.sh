#!/bin/sh

cd /go/src//modules/audit/src && go mod tidy

cd /go/src//modules/auth/src && go mod tidy

cd /go/src//modules/queue/src && go mod tidy

cd /go/src//modules/shared/src && go mod tidy
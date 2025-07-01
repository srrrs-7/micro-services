FROM --platform=amd64 golang:latest AS builder
RUN curl -sSf https://atlasgo.sh | sh

FROM debian:latest
COPY --from=builder /usr/local/bin/atlas /usr/local/bin/atlas

ENTRYPOINT [ "/usr/local/bin/atlas" ]
FROM golang:1.20-alpine3.17 AS build
WORKDIR /go/nginx-p2p-cache
COPY . .
RUN set -x \
    && CGO_ENABLED=0 go build -o nginx-p2p-cache .

FROM alpine:3.17
COPY --from=build /go/nginx-p2p-cache/nginx-p2p-cache /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/nginx-p2p-cache"]

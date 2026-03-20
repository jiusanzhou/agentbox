FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 go build -ldflags '-s -w -extldflags "-static"' -o /bin/abox ./cmd/abox
RUN CGO_ENABLED=1 go build -ldflags '-s -w -extldflags "-static"' -o /bin/aboxctl ./cmd/aboxctl

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs

RUN adduser -D -u 1000 abox

COPY --from=builder /bin/abox /usr/local/bin/abox
COPY --from=builder /bin/aboxctl /usr/local/bin/aboxctl

RUN mkdir -p /data && chown abox:abox /data
VOLUME /data

USER abox

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/healthz || exit 1

ENTRYPOINT ["abox"]
CMD ["--config", "/etc/abox/config.yaml"]

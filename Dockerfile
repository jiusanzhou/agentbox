FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 go build -ldflags '-s -w' -o /bin/abox ./cmd/abox

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=builder /bin/abox /usr/local/bin/abox

RUN mkdir -p /data
VOLUME /data

EXPOSE 8080

ENTRYPOINT ["abox"]
CMD ["--config", "/etc/abox/config.yaml"]

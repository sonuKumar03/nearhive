FROM golang:alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o nearhive ./cmd/nearhive

# Final runtime image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /build/nearhive /usr/local/bin/nearhive
COPY --from=builder /build/migrations /app/migrations
COPY --from=builder /build/config /app/config

EXPOSE 8080

ENTRYPOINT ["nearhive"]
CMD ["serve"]

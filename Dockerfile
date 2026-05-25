FROM golang:1.22-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mywebapp ./cmd/mywebapp

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/mywebapp .
COPY --from=builder /app/migrations/ migrations/
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 5200

ENTRYPOINT ["./entrypoint.sh"]

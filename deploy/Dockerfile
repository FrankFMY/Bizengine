FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/engine ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/engine /bin/engine
COPY migrations /app/migrations
COPY processes /app/processes
WORKDIR /app
ENTRYPOINT ["/bin/engine"]

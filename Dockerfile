# Stage 1: Build the application
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/fluxgate .

# Stage 2: Run the application
FROM alpine:3.22

RUN apk --no-cache add ca-certificates

COPY --from=builder /out/fluxgate /usr/local/bin/fluxgate

ENTRYPOINT ["fluxgate"]
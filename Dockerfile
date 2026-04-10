FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/listener ./cmd/listener

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /out/listener /app/listener

ENTRYPOINT ["/app/listener"]

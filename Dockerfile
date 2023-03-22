FROM golang:1.20-alpine3.17 AS builder
COPY . /app
WORKDIR /app
RUN go build -o /app/main ./cmd/contenttruck

FROM alpine:3.17.2
COPY --from=builder /app/main /app/main
WORKDIR /app
RUN apk add --no-cache ca-certificates
CMD ["/app/main"]

FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git gcc musl-dev
WORKDIR /build
COPY go/ .
COPY web/ /app/web/
RUN go mod tidy
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server/

FROM alpine:3.19
RUN apk --no-cache add ca-certificates \
    && addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /server /app/server
COPY --from=builder /app/web/ /app/web/
COPY go/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
WORKDIR /app
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/app/server"]

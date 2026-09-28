FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 1000 app
COPY --from=builder /out/app /app
USER app
ENTRYPOINT ["/app"]
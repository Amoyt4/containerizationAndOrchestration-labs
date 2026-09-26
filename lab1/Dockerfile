FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/api main.go

FROM scratch
COPY --from=builder /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
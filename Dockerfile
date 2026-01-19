FROM golang:1.25.4-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o blacklist .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/blacklist .
EXPOSE 8888
CMD ["./blacklist"]
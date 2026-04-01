FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o service-a .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/service-a /usr/local/bin/service-a
EXPOSE 8080
CMD ["service-a"]

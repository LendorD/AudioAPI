FROM golang:1.24-alpine AS builder

RUN apk update && apk add --no-cache ca-certificates git openssh

WORKDIR /app

COPY . .

RUN go build -o app ./cmd/app/main.go

RUN chmod +x app

EXPOSE 8080

ENTRYPOINT ["./app"]

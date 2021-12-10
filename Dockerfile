FROM golang:1.17-alpine as builder

WORKDIR /app

RUN apk update && \
    apk add gcc musl-dev upx

COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
RUN go build -o ascii-ssh-movie main.go
RUN upx -q -9 ascii-ssh-movie
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/ascii-ssh-movie .
COPY data ./data
ENTRYPOINT [ "./ascii-ssh-movie" ]
FROM golang:alpine AS builder

WORKDIR /app

COPY ./app/go.mod ./app/go.sum ./

RUN go mod download && go mod verify

COPY ./app .

RUN go build -o app .

FROM alpine:latest as runner

WORKDIR /app
COPY --from=builder /app/app .

EXPOSE 8080

CMD ["./app"]
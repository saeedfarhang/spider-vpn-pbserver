# syntax=docker/dockerfile:1

FROM golang:1.22.0

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /docker-gs-ping

EXPOSE ${APP_PORT}

CMD ["sh", "-c", "/docker-gs-ping serve --http=${APP_HOST}:${APP_PORT}"]

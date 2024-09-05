# syntax=docker/dockerfile:1

FROM golang:1.22.0 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /docker-gs-ping

FROM golang:1.22.0 AS app

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=builder /docker-gs-ping /docker-gs-ping

EXPOSE ${APP_PORT}

CMD ["sh", "-c", "/docker-gs-ping serve --http=${APP_HOST}:${APP_PORT}"]

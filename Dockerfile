# syntax=docker/dockerfile:1

FROM golang:1.22.0

# Change proxy URL to use localhost
# ENV http_proxy=http://localhost:20171
# ENV HTTP_PROXY=http://localhost:20171
# ENV https_proxy=http://localhost:20171
# ENV HTTPS_PROXY=http://localhost:20171

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /docker-gs-ping

EXPOSE 8090

CMD ["/docker-gs-ping", "serve"]

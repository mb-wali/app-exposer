### First stage
FROM golang:1.12 as build-root

WORKDIR /build

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go install -v ./...

ENTRYPOINT ["app-exposer"]

EXPOSE 60000

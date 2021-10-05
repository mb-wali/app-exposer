### First stage
FROM quay.io/goswagger/swagger as swagger

FROM golang:1.16 as build-root

WORKDIR /build

COPY go.mod .
COPY go.sum .
COPY --from=swagger /usr/bin/swagger /usr/bin/

COPY . .

# copy config file 
COPY jobservices.yml /etc/iplant/de/jobservices.yml
# COPY config /root/.kube/config

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go install -v ./...
RUN swagger generate spec -o ./docs/swagger.json --scan-models

ENTRYPOINT ["app-exposer"]

EXPOSE 60009


# build
# docker build -t mbwali/app-exposer:latest .

# run
# docker run -it -p 60009:60009 mbwali/app-exposer:latest

# config
# /config/default.yml



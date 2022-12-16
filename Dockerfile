# syntax=docker/dockerfile:1

FROM golang:1.19-alpine as builder

WORKDIR /tmp/app

COPY go.mod ./
COPY go.sum ./
RUN go mod download
RUN go mod verify
COPY *.go ./

RUN go build -o ./wilson-art-csv-gen

FROM alpine:3.17

COPY --from=builder /tmp/app/wilson-art-csv-gen /app/wilson-art-csv-gen

EXPOSE 8080

CMD [ "./app/wilson-art-csv-gen" ]
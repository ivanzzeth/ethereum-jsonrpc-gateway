FROM golang:1.22-alpine3.20 AS builder

ENV GO111MODULE=on \
    GOPROXY=https://goproxy.io,direct

WORKDIR /usr/src/app

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download && go mod verify

COPY . /usr/src/app
RUN go build -v -installsuffix cgo -ldflags '-s -w' -o /usr/local/bin/app .

FROM alpine:3.20
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
RUN apk --no-cache add ca-certificates
COPY --from=builder /usr/local/bin/app /usr/local/bin/app
COPY --from=builder /usr/src/app/config.public.json /usr/src/app/config.json

WORKDIR /usr/src/app

RUN addgroup -S appuser && adduser -S -G appuser appuser
USER appuser
ENTRYPOINT [ "app" ]
CMD [ "start" ]

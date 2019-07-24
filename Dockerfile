FROM golang:1.12

WORKDIR /go/src/ddex/ethereum-jsonrpc-gateway
COPY . /go/src/ddex/ethereum-jsonrpc-gateway
RUN make build

FROM alpine
RUN apk --no-cache add ca-certificates
COPY --from=0 /go/src/ddex/ethereum-jsonrpc-gateway/bin/cli /bin/
CMD ["cli"]

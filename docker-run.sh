#!/bin/sh
docker rm -f ethereum-jsonrpc-gateway

docker run -d --name ethereum-jsonrpc-gateway -p 3005:3005 \
    -v ./config.json:/usr/src/app/config.json \
    --restart always \
    ivanzz/ethereum-jsonrpc-gateway

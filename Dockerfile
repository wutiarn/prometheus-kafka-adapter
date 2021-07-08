FROM golang:1.16.5-alpine3.14 as build

WORKDIR /src/prometheus-kafka-adapter

RUN apk add --no-cache librdkafka=1.7.0-r0 gcc

COPY go.mod .
COPY go.sum .
RUN go mod download

ADD . /src/prometheus-kafka-adapter

RUN go build -o /prometheus-kafka-adapter
RUN go test ./...

FROM alpine:3.14

# Static linking of librdkafka breaks DNS: https://github.com/segmentio/kafka-go/issues/285
# %3|1625743853.672|FAIL|rdkafka#producer-1| [thrd:ssl://example.host.tld:9093/bootstrap]: ssl://example.host.tld:9093/bootstrap: Failed to resolve 'example.host.tld:9093': Device or resource busy (after 0ms in state CONNECT, 15 identical error(s) suppressed)
RUN apk add --no-cache 'librdkafka=1.7.0-r0'

COPY schemas/metric.avsc /schemas/metric.avsc
COPY --from=build /prometheus-kafka-adapter /

CMD /prometheus-kafka-adapter

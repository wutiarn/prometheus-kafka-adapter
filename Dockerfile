FROM golang:1.16.3-buster as build

WORKDIR /src/prometheus-kafka-adapter

COPY go.mod .
COPY go.sum .
RUN go mod download

ADD . /src/prometheus-kafka-adapter

RUN go build -o /prometheus-kafka-adapter
RUN go test ./...

FROM ubuntu:20.04
# librdkafka is pre-built for glibc-based distributions (like RedHat, Debian, CentOS, Ubuntu), and Alpine is musl-based.
# See confluent-kafka-go README for details: https://github.com/confluentinc/confluent-kafka-go

COPY schemas/metric.avsc /schemas/metric.avsc
COPY --from=build /prometheus-kafka-adapter /

CMD /prometheus-kafka-adapter

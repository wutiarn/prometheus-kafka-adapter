// Copyright 2018 Telefónica
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/golang/snappy"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/prometheus/prompb"
)

func receiveHandler(
	producer *kafka.Producer,
	serializer Serializer,
) func(c *gin.Context) {
	return func(c *gin.Context) {

		httpRequestsTotal.Add(float64(1))

		compressed, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			logrus.WithError(err).Error("couldn't read body")
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			logrus.WithError(err).Error("couldn't decompress body")
			return
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			logrus.WithError(err).Error("couldn't unmarshal body")
			return
		}

		metricsPerTopic, err := processWriteRequest(&req)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			logrus.WithError(err).Error("couldn't process write request")
			return
		}

		producerDeliveryReportsChan := make(chan kafka.Event)
		sentMessagesCount := 0

		for topic, metrics := range metricsPerTopic {
			t := topic
			part := kafka.TopicPartition{
				Partition: kafka.PartitionAny,
				Topic:     &t,
			}
			for _, metric := range metrics {
				err := producer.Produce(&kafka.Message{
					TopicPartition: part,
					Value:          metric,
				}, producerDeliveryReportsChan)

				if err != nil {
					c.AbortWithStatus(http.StatusInternalServerError)
					logrus.WithError(err).Error("couldn't produce message in kafka")
					return
				}
				sentMessagesCount++
			}
		}

		timeoutChan := time.After(kafkaProducerTimeout)
		for i := 0; i < sentMessagesCount; {
			select {
			case <-producerDeliveryReportsChan:
				i++
			case <-timeoutChan:
				// Note that producerDeliveryReportsChan must remain open: otherwise next delivery report will cause
				// unhandled panic (and consequent process termination). Eventually it will be garbage collected.
				//c.AbortWithStatus(http.StatusTooManyRequests)
				logrus.WithError(err).Error("Kafka message producer timed out")
				return
			}
		}
	}
}

/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockcutter

import (
	"fmt"
	"github.com/hyperledger/fabric/orderer/common/prometheus"
	"math/rand"
	"time"

	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/flogging"
)

var logger = flogging.MustGetLogger("orderer.common.blockcutter")

type OrdererConfigFetcher interface {
	OrdererConfig() (channelconfig.Orderer, bool)
}

type ControllerDataFetcher interface {
	ControllerConfig() (prometheus.ControllerInterface, bool)
}

// Receiver defines a sink for the ordered broadcast messages
type Receiver interface {
	// Ordered should be invoked sequentially as messages are ordered
	// Each batch in `messageBatches` will be wrapped into a block.
	// `pending` indicates if there are still messages pending in the receiver.
	Ordered(msg *cb.Envelope, controller *prometheus.Controller) (messageBatches [][]*cb.Envelope, pending bool)

	// Cut returns the current batch and starts a new one
	Cut() []*cb.Envelope
}

type receiver struct {
	sharedConfigFetcher   OrdererConfigFetcher
	pendingBatch          []*cb.Envelope
	pendingBatchSizeBytes uint32

	PendingBatchStartTime time.Time
	ChannelID             string
	Metrics               *Metrics
}

// NewReceiverImpl creates a Receiver implementation based on the given configtxorderer manager
func NewReceiverImpl(channelID string, sharedConfigFetcher OrdererConfigFetcher, metrics *Metrics) Receiver {
	return &receiver{
		sharedConfigFetcher: sharedConfigFetcher,
		Metrics:             metrics,
		ChannelID:           channelID,
	}
}

func randU32(min, max uint32) uint32 {
	var a = rand.Uint32()
	a %= max - min
	a += min
	return a
}

// Ordered should be invoked sequentially as messages are ordered
//
// messageBatches length: 0, pending: false
//   - impossible, as we have just received a message
// messageBatches length: 0, pending: true
//   - no batch is cut and there are messages pending
// messageBatches length: 1, pending: false
//   - the message count reaches BatchSize.MaxMessageCount
// messageBatches length: 1, pending: true
//   - the current message will cause the pending batch size in bytes to exceed BatchSize.PreferredMaxBytes.
// messageBatches length: 2, pending: false
//   - the current message size in bytes exceeds BatchSize.PreferredMaxBytes, therefore isolated in its own batch.
// messageBatches length: 2, pending: true
//   - impossible
//
// Note that messageBatches can not be greater than 2.
func (r *receiver) Ordered(msg *cb.Envelope, controller *prometheus.Controller) (messageBatches [][]*cb.Envelope, pending bool) {
	if len(r.pendingBatch) == 0 {
		// We are beginning a new batch, mark the time
		r.PendingBatchStartTime = time.Now()
	}
	ordererConfig, ok := r.sharedConfigFetcher.OrdererConfig()
	if !ok {
		logger.Panicf("Could not retrieve orderer config to query batch parameters, block cutting is not possible")
	}
	// batchSize := ordererConfig.BatchSize() // TODO: replace the static with a dynamic version (batch, block size)

	batchSize := ordererConfig.BatchSize()
	fmt.Println("UPDATED batch size ", batchSize)
	bytes := messageSizeBytes(msg)

	// TODO: First Condition
	if bytes > batchSize.PreferredMaxBytes {
		newBatchSize, ok := controller.Run(ordererConfig.BatchSize(), prometheus.PreferredMaxBytes, bytes)
		if ok == true {
			ordererConfig.BatchSize().PreferredMaxBytes = newBatchSize.PreferredMaxBytes
			fmt.Println("Updated attribute: ", ordererConfig.BatchSize().PreferredMaxBytes)
		}
		// TODO: increase max bytes on next iteration (?maybe)
		fmt.Println("Message Size Bytes larger than preferred max bytes ", bytes)
		logger.Debugf("The current message, with %v bytes, is larger than the preferred batch size of %v bytes and will be isolated.", bytes, batchSize.PreferredMaxBytes)
		fmt.Println("size bytes pending batch ", r.pendingBatchSizeBytes)
		// cut pending batch, if it has any messages
		if len(r.pendingBatch) > 0 {
			fmt.Println("cutting here")
			messageBatch := r.Cut()
			fmt.Println("cutting message batch ", messageBatch)
			messageBatches = append(messageBatches, messageBatch)
		}
		// TODO: increase PreferredMaxBytes batch size here (going to be used in the next block cut)

		// create new batch with single message
		messageBatches = append(messageBatches, []*cb.Envelope{msg})

		// Record that this batch took no time to fill
		r.Metrics.BlockFillDuration.With("channel", r.ChannelID).Observe(0)
		return
	}

	// TODO: Second Condition
	messageWillOverflowBatchSizeBytes := r.pendingBatchSizeBytes+bytes > batchSize.PreferredMaxBytes
	if messageWillOverflowBatchSizeBytes {
		logger.Debugf("The current message, with %v bytes, will overflow the pending batch of %v bytes.", bytes, r.pendingBatchSizeBytes)
		logger.Debugf("Pending batch would overflow if current message is added, cutting batch now.")
		messageBatch := r.Cut()
		r.PendingBatchStartTime = time.Now()
		messageBatches = append(messageBatches, messageBatch)
	}

	logger.Debugf("Enqueuing message into batch")
	r.pendingBatch = append(r.pendingBatch, msg)
	r.pendingBatchSizeBytes += bytes
	fmt.Println("SIZE IN BYTES OF PENDING BATCH ", bytes)
	fmt.Println("LENGTH OF BATCHES BEFORE CUTTING ", len(messageBatches))
	pending = true

	// TODO: Third condition
	if uint32(len(r.pendingBatch)) >= batchSize.MaxMessageCount {
		newBatchSize, ok := controller.Run(ordererConfig.BatchSize(), prometheus.MaxMessageCount, bytes)
		if ok == true {
			ordererConfig.BatchSize().MaxMessageCount = newBatchSize.MaxMessageCount
			fmt.Println("Updated attribute: ", ordererConfig.BatchSize().MaxMessageCount)
		}
		logger.Debugf("Batch size met, cutting batch")
		messageBatch := r.Cut()
		messageBatches = append(messageBatches, messageBatch)
		fmt.Println("message batch just cut", "MESSAGE BATCHES LENGTH", len(messageBatches)) //TODO: Output length
		pending = false
		// TODO: Increase MaxMessageCount on next iteration

	}
	return
}

// Cut returns the current batch and starts a new one
func (r *receiver) Cut() []*cb.Envelope {
	if r.pendingBatch != nil {
		r.Metrics.BlockFillDuration.With("channel", r.ChannelID).Observe(time.Since(r.PendingBatchStartTime).Seconds())
	}
	r.PendingBatchStartTime = time.Time{}
	batch := r.pendingBatch
	r.pendingBatch = nil
	r.pendingBatchSizeBytes = 0
	return batch
}

func messageSizeBytes(message *cb.Envelope) uint32 {
	return uint32(len(message.Payload) + len(message.Signature))
}

func customMessageSizeBytes(message *cb.Envelope) uint32 {
	fmt.Println(uint32(len(message.Payload)-len(message.Signature)), "KB")
	return uint32(len(message.Payload) - len(message.Signature))
}

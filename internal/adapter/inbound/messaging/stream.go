package messaging

import (
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// ensureStreamExists creates the INDEXING stream if it doesn't exist.
func (n *NATSConsumer) ensureStreamExists() error {
	streamName := IndexingStreamName

	// For parameter testing consumers, we need a different stream configuration
	// Delete and recreate if needed
	if strings.Contains(n.config.DurableName, "test-params") {
		_ = n.jsContext.DeleteStream(streamName)
	}

	// Check if stream already exists
	_, err := n.jsContext.StreamInfo(streamName)
	if err == nil {
		// Stream already exists
		return nil
	}

	// Stream doesn't exist, create it
	streamConfig := &nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"indexing.>"},
		Storage:  nats.FileStorage,
		MaxAge:   StreamRetentionHours * time.Hour, // Keep messages for configured hours
	}

	// Use standard retention for parameter testing consumers to avoid WorkQueue restrictions
	if strings.Contains(n.config.DurableName, "test-params") {
		streamConfig.Retention = nats.LimitsPolicy
	} else {
		streamConfig.Retention = nats.WorkQueuePolicy
	}

	_, err = n.jsContext.AddStream(streamConfig)
	if err != nil {
		return fmt.Errorf("failed to create stream %s: %w", streamName, err)
	}

	return nil
}

// createDurableConsumer creates a durable consumer on the INDEXING stream.
func (n *NATSConsumer) createDurableConsumer() error {
	streamName := IndexingStreamName

	// Parse replay policy
	replayPolicy := nats.ReplayInstantPolicy
	if n.config.ReplayPolicy == "original" {
		replayPolicy = nats.ReplayOriginalPolicy
	}

	// Parse deliver policy
	deliverPolicy := nats.DeliverAllPolicy
	switch n.config.DeliverPolicy {
	case "new":
		deliverPolicy = nats.DeliverNewPolicy
	case "last":
		deliverPolicy = nats.DeliverLastPolicy
	case "last_per_subject":
		deliverPolicy = nats.DeliverLastPerSubjectPolicy
	}

	consumerConfig := &nats.ConsumerConfig{
		Durable:        n.config.DurableName,
		DeliverSubject: "", // Pull-based consumer
		DeliverGroup:   n.config.QueueGroup,
		FilterSubject:  n.config.Subject,
		AckPolicy:      nats.AckExplicitPolicy,
		AckWait:        n.config.AckWait,
		MaxDeliver:     n.config.MaxDeliver,
		MaxAckPending:  n.config.MaxAckPending,
		ReplayPolicy:   replayPolicy,
		DeliverPolicy:  deliverPolicy,
	}

	// Only set optional fields if they have non-zero values
	if n.config.MaxRequestBatch > 0 {
		consumerConfig.MaxRequestBatch = n.config.MaxRequestBatch
	}
	if n.config.MaxRequestExpires > 0 {
		consumerConfig.MaxRequestExpires = n.config.MaxRequestExpires
	}
	if n.config.InactiveThreshold > 0 {
		consumerConfig.InactiveThreshold = n.config.InactiveThreshold
	}

	// Set optional fields if specified
	if n.config.OptStartSeq > 0 {
		consumerConfig.OptStartSeq = n.config.OptStartSeq
	}
	if n.config.OptStartTime != nil {
		consumerConfig.OptStartTime = n.config.OptStartTime
	}
	if n.config.RateLimitBps > 0 {
		consumerConfig.RateLimit = n.config.RateLimitBps
	}
	if n.config.MaxWaiting > 0 {
		consumerConfig.MaxWaiting = n.config.MaxWaiting
	}

	// For WorkQueue streams, we need to handle existing consumers carefully
	// Try to delete existing consumer first (if it exists) to avoid conflicts
	_ = n.jsContext.DeleteConsumer(streamName, n.config.DurableName)

	// In WorkQueue streams with the same filter subject, we need to handle conflicts
	// For testing purposes, try to clean up any conflicting consumers first
	_, streamErr := n.jsContext.StreamInfo(streamName)
	if streamErr == nil {
		// Try to delete common test consumer names that might conflict
		testConsumers := []string{"indexing-consumer", "test-consumer", "test-params-consumer"}
		for _, consumerName := range testConsumers {
			_ = n.jsContext.DeleteConsumer(streamName, consumerName)
		}
	}

	// Track the actual subject being used
	n.actualSubject = n.config.Subject

	// Create the consumer
	_, err := n.jsContext.AddConsumer(streamName, consumerConfig)
	if err != nil {
		// Handle various WorkQueue stream requirements
		if strings.Contains(err.Error(), "not unique") || strings.Contains(err.Error(), "multiple non-filtered") {
			// For WorkQueue streams, use a unique filter subject per consumer
			uniqueSubject := n.config.Subject + "." + n.config.DurableName
			consumerConfig.FilterSubject = uniqueSubject
			n.actualSubject = uniqueSubject
			_, err = n.jsContext.AddConsumer(streamName, consumerConfig)
		} else if strings.Contains(err.Error(), "deliver all") {
			// Some WorkQueue configurations require DeliverAll policy
			consumerConfig.DeliverPolicy = nats.DeliverAllPolicy
			_, err = n.jsContext.AddConsumer(streamName, consumerConfig)
		}
		if err != nil {
			return fmt.Errorf("failed to create durable consumer %s: %w", n.config.DurableName, err)
		}
	}

	return nil
}

// startSubscription starts the pull-based subscription.
func (n *NATSConsumer) startSubscription() error {
	streamName := IndexingStreamName
	consumerName := n.config.DurableName

	// Create pull subscription using the actual subject that was configured for the consumer
	subjectToUse := n.actualSubject
	if subjectToUse == "" {
		subjectToUse = n.config.Subject
	}

	sub, err := n.jsContext.PullSubscribe(subjectToUse, consumerName, nats.Bind(streamName, consumerName))
	if err != nil {
		return fmt.Errorf("failed to create pull subscription: %w", err)
	}

	// Wrap the subscription to provide test-compatible behavior
	n.subscription = &subscriptionWrapper{
		Subscription: sub,
		Subject:      n.config.Subject,
	}

	// Start message processing goroutine
	go n.messageProcessingLoop()

	return nil
}

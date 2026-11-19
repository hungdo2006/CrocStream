// Package latest supports bounded queues for replaceable snapshots.
package latest

// Offer enqueues value without blocking and reports whether a snapshot was
// dropped. The queue must have capacity one and serialized producers.
func Offer[T any](queue chan T, value T) bool {
	select {
	case queue <- value:
		return false
	default:
	}

	dropped := false
	select {
	case <-queue:
		dropped = true
	default:
	}
	select {
	case queue <- value:
		return dropped
	default:
		return true
	}
}

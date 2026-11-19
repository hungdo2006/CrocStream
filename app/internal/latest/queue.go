// Package latest supports bounded queues for replaceable snapshots.
package latest

// Offer enqueues value and reports whether a snapshot was dropped. Use a
// capacity-one channel with one producer (or serialized producers). The
// consumer may drain the channel between the first send and the eviction.
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

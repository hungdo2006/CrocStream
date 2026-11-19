package latest

import "testing"

func TestOfferKeepsNewestSnapshot(t *testing.T) {
	queue := make(chan int, 1)
	for value := 0; value < 100; value++ {
		if dropped := Offer(queue, value); dropped != (value > 0) {
			t.Fatalf("Offer(%d) dropped = %t", value, dropped)
		}
	}
	if len(queue) != 1 {
		t.Fatalf("queue size = %d, want 1", len(queue))
	}
	if value := <-queue; value != 99 {
		t.Fatalf("queued snapshot = %d, want 99", value)
	}
	if Offer(queue, 100) {
		t.Fatal("offering to a drained queue dropped a snapshot")
	}
}

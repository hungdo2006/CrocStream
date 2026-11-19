package latest

import "testing"

func TestOfferKeepsNewestSnapshot(t *testing.T) {
	queue := make(chan int, 1)
	for value := 0; value < 100; value++ {
		if dropped := Offer(queue, value); dropped != (value > 0) {
			t.Fatalf("Offer(%d) dropped = %t", value, dropped)
		}
	}
	if len(queue) != 1 || <-queue != 99 {
		t.Fatal("queue did not keep the newest snapshot")
	}
	if Offer(queue, 100) {
		t.Fatal("offering after the consumer drained the queue counted a drop")
	}
}

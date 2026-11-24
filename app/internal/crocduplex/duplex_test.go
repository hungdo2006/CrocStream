package crocduplex

import (
	"context"
	"testing"
)

func TestFailedJoinReturnsNilInterface(t *testing.T) {
	stream, err := Join(context.Background(), "invalid", Options{RelayAddress: "127.0.0.1:1", RelayPassword: "private"})
	if err == nil || stream != nil {
		t.Fatal("failed join returned typed-nil stream")
	}
}

package transport

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestOptionsValidate(t *testing.T) {
	valid := Options{Address: "127.0.0.1:9009", Password: "valid-relay-password"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid options, got %v", err)
	}

	cases := []struct {
		name string
		opts Options
	}{
		{"empty address", Options{Address: "", Password: "secret"}},
		{"missing port", Options{Address: "127.0.0.1", Password: "secret"}},
		{"missing host", Options{Address: ":9009", Password: "secret"}},
		{"port zero", Options{Address: "127.0.0.1:0", Password: "secret"}},
		{"port too high", Options{Address: "127.0.0.1:65536", Password: "secret"}},
		{"non-numeric port", Options{Address: "127.0.0.1:http", Password: "secret"}},
		{"oversized address", Options{Address: strings.Repeat("a", 296) + ":9009", Password: "secret"}},
		{"empty password", Options{Address: "127.0.0.1:9009", Password: ""}},
		{"oversized password", Options{Address: "127.0.0.1:9009", Password: strings.Repeat("p", 1025)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.opts.Validate(); err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
			if _, err := (Croc{}).Host(context.Background(), tc.opts, nil); err == nil {
				t.Fatalf("Host accepted invalid options for %s", tc.name)
			}
			if _, err := (Croc{}).Join(context.Background(), "cs1-code", tc.opts, nil); err == nil {
				t.Fatalf("Join accepted invalid options for %s", tc.name)
			}
		})
	}
}

type fakeProgressListener struct {
	progressed bool
}

func (f *fakeProgressListener) Invitation() string { return "cs1-fake" }
func (f *fakeProgressListener) Close() error       { return nil }
func (f *fakeProgressListener) Accept(context.Context) (io.ReadWriteCloser, error) {
	return nil, io.EOF
}
func (f *fakeProgressListener) AcceptWithProgress(_ context.Context, progress func()) (io.ReadWriteCloser, error) {
	if progress != nil {
		progress()
	}
	f.progressed = true
	return nil, io.EOF
}

func TestListenerProgressForwardsCallback(t *testing.T) {
	inner := &fakeProgressListener{}
	called := false
	wrapped := &listenerProgress{
		Listener: inner,
		progress: func() { called = true },
	}
	_, err := wrapped.Accept(context.Background())
	if err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inner.progressed || !called {
		t.Fatalf("expected AcceptWithProgress to invoke progress callback")
	}
}

type fakeRoutedConn struct{ route string }

func (fakeRoutedConn) Read([]byte) (int, error)  { return 0, io.EOF }
func (fakeRoutedConn) Write([]byte) (int, error) { return 0, io.EOF }
func (fakeRoutedConn) Close() error              { return nil }
func (c fakeRoutedConn) Route() string           { return c.route }

func TestOptionsEnableLANAndRouteOf(t *testing.T) {
	got := options(Options{Address: "198.51.100.10:9009", Password: "example-relay-pass"})
	if !got.EnableLAN {
		t.Fatal("expected Croc transport options to enable LAN-first discovery")
	}
	if r := RouteOf(fakeRoutedConn{route: "lan"}); r != "lan" {
		t.Fatalf("expected lan route, got %q", r)
	}
	if r := RouteOf(nil); r != "" {
		t.Fatalf("expected empty route for nil conn, got %q", r)
	}
}

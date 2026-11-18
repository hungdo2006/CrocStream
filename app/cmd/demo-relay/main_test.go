package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestBoundedSetup(t *testing.T) {
	for _, text := range []string{"", "localhost:9009\nlong-password\n", "0.0.0.0:0\nlong-password\n", "8.8.8.8:9009\nlong-password\n",
		"127.0.0.1:9009\nshort\n", "127.0.0.1:9009\n" + strings.Repeat("a", 2048) + "\n",
		"127.0.0.1:9009\n password-with-space \n"} {
		if _, _, err := readSetup(bufio.NewScanner(strings.NewReader(text))); err == nil {
			t.Fatal("invalid setup accepted")
		}
	}
	if a, p, err := readSetup(bufio.NewScanner(strings.NewReader("127.0.0.1:9009\nlong-password\n"))); err != nil || a != "127.0.0.1:9009" || p != "long-password" {
		t.Fatal("valid setup rejected")
	}
}

func TestRelayStdinStopClosesListener(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	input, writer := io.Pipe()
	out, output := io.Pipe()
	defer input.Close()
	defer writer.Close()
	defer out.Close()
	defer output.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, input, output) }()
	go func() { fmt.Fprintf(writer, "%s\nprivate-test-password\n", addr) }()
	scanner := bufio.NewScanner(out)
	if !scanner.Scan() || strings.Contains(scanner.Text(), "private") {
		t.Fatal("readiness/privacy")
	}
	writer.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("relay did not stop")
	}
	conn, err := net.DialTimeout("tcp4", addr, time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("listener still open")
	}
}

func TestCancellationUnblocksIncompletePrivateInput(t *testing.T) {
	for _, prefix := range []string{"", "127.0.0.1:9009\n"} {
		input, writer := io.Pipe()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- run(ctx, input, io.Discard) }()
		if prefix != "" {
			_, _ = io.WriteString(writer, prefix)
		}
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("cancellation leaked setup reader")
		}
		writer.Close()
	}
}

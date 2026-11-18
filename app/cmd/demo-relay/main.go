// demo-relay wraps the pinned upstream relay for a manually operated demo.
// Its password arrives only on stdin; there are no command-line secrets.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/schollz/croc/v11/src/logger"
	"github.com/schollz/croc/v11/src/tcp"
)

func readSetup(r *bufio.Scanner) (string, string, error) {
	r.Buffer(make([]byte, 256), 2048)
	if !r.Scan() {
		return "", "", errors.New("missing address")
	}
	address := r.Text()
	host, port, err := net.SplitHostPort(address)
	n, parseErr := strconv.Atoi(port)
	ip := net.ParseIP(host)
	if err != nil || parseErr != nil || ip.To4() == nil || (!ip.IsLoopback() && !ip.IsPrivate()) || n < 1 || n > 65535 {
		return "", "", errors.New("listen only on loopback or a private IPv4 interface")
	}
	if !r.Scan() {
		return "", "", errors.New("missing password")
	}
	password := r.Text()
	if len(password) < 12 || len(password) > 1024 || strings.TrimSpace(password) != password {
		return "", "", errors.New("password must be 12..1024 bytes without edge spaces")
	}
	return address, password, nil
}

// run owns input and closes it to unblock the command reader on cancellation.
func run(ctx context.Context, input io.ReadCloser, output io.Writer) error {
	defer input.Close()
	stopInput := context.AfterFunc(ctx, func() { _ = input.Close() })
	defer stopInput()
	scanner := bufio.NewScanner(input)
	address, password, err := readSetup(scanner)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp4", address)
	if err != nil {
		return errors.New("cannot bind relay address")
	}
	defer ln.Close()
	logger.SetOutput(io.Discard)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Closing stdin or writing any further line stops this demo process.
	readerDone := make(chan struct{})
	go func() { defer close(readerDone); scanner.Scan(); cancel() }()
	defer func() { input.Close(); <-readerDone }()
	fmt.Fprintln(output, "Relay ready. Press Enter in the launcher to stop.")
	err = tcp.RunWithListener(ln, password, tcp.WithCtx(ctx), tcp.WithLogLevel("error"))
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Relay stopped: check listen address, port and password (12..1024 bytes).")
		os.Exit(1)
	}
}

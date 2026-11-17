// G4 proof peers carry synthetic RGBA only. Setup/invitation travel in private
// stdin/stdout pipes; direct child mode stdout must never be persisted as a log.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/protocol"
	"crocstream.local/app/internal/session"
	"crocstream.local/app/internal/transport"
	"github.com/schollz/croc/v11/src/tcp"
)

type setup struct {
	Address, Password, Invite string
	Duration                  time.Duration
}
type result struct {
	Role            string           `json:"role"`
	Counters        session.Counters `json:"counters"`
	LastHash        string           `json:"lastHash"`
	ActiveSeconds   float64          `json:"activeSeconds"`
	GoroutinesStart int              `json:"goroutinesStart"`
	GoroutinesEnd   int              `json:"goroutinesEnd"`
}

func frame(tick int) capture.Frame {
	b := make([]byte, 640*360*4)
	for i := 3; i < len(b); i += 4 {
		b[i] = 255
	}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			p := (y*640 + x) * 4
			b[p], b[p+1], b[p+2] = byte(tick*17), byte(tick*31), byte(tick*47)
		}
	}
	return capture.Frame{RGBA: b, Width: 640, Height: 360, At: time.Now()}
}
func peer(mode string) error {
	var input setup
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 8192)).Decode(&input); err != nil {
		return errors.New("invalid private setup")
	}
	if input.Duration < time.Second || input.Duration > 30*time.Minute {
		return errors.New("duration out of bounds")
	}
	ctx, cancel := context.WithTimeout(context.Background(), input.Duration+45*time.Second)
	defer cancel()
	factory := transport.Croc{}
	options := transport.Options{Address: input.Address, Password: input.Password}
	if mode == "relay" {
		ln, err := net.Listen("tcp4", input.Address)
		if err != nil {
			return err
		}
		defer ln.Close()
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"event": "relay_ready", "address": ln.Addr().String()})
		return tcp.RunWithListener(ln, input.Password, tcp.WithCtx(ctx), tcp.WithLogLevel("error"))
	}
	var conn io.ReadWriteCloser
	var err error
	if mode == "host" {
		listener, e := factory.Host(ctx, options, nil)
		if e != nil {
			return e
		}
		defer listener.Close()
		// Private IPC only: parent forwards this line and never prints it.
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"Invite": listener.Invitation()})
		conn, err = listener.Accept(ctx)
	} else {
		conn, err = factory.Join(ctx, input.Invite, options, nil)
	}
	input.Password = ""
	input.Invite = ""
	options.Password = ""
	if err != nil {
		return err
	}
	defer conn.Close()
	metrics := session.NewMetrics()
	var hashMu sync.Mutex
	var lastHash string
	var badImage atomic.Bool
	hooks := session.Hooks{Reference: func(id uint32, rgba []byte) {
		expected := sha256.Sum256(frame(int(id) - 1).RGBA)
		hash := sha256.Sum256(rgba)
		hashMu.Lock()
		lastHash = hex.EncodeToString(hash[:])
		hashMu.Unlock()
		if expected != hash {
			badImage.Store(true)
			cancel()
		}
	}}
	p := session.NewPeer(conn, mode == "host", metrics, hooks)
	cfg := protocol.DefaultConfig()
	cfg.Width, cfg.Height = 640, 360
	var frames chan capture.Frame
	producerDone := make(chan struct{})
	if mode == "host" {
		frames = make(chan capture.Frame, 1)
		go func() {
			defer close(producerDone)
			defer close(frames)
			timer := time.NewTimer(input.Duration)
			defer timer.Stop()
			ticker := time.NewTicker(time.Second / 24)
			defer ticker.Stop()
			for i := 0; ; i++ {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					return
				case <-ticker.C:
				}
				select {
				case frames <- frame(i):
				case <-ctx.Done():
					return
				}
			}
		}()
	} else {
		go func() {
			defer close(producerDone)
			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				p.RequestRecovery()
			}
		}()
	}
	startG := runtime.NumGoroutine()
	start := time.Now()
	err = p.Run(ctx, cfg, frames)
	duration := time.Since(start).Seconds()
	cancel()
	<-producerDone
	if err != nil {
		return err
	}
	if badImage.Load() {
		return errors.New("synthetic canonical mismatch")
	}
	count, _ := metrics.Read()
	hashMu.Lock()
	hash := lastHash
	hashMu.Unlock()
	return json.NewEncoder(os.Stdout).Encode(result{mode, count, hash, duration, startG, runtime.NumGoroutine()})
}
func verify(results []result, duration time.Duration) error {
	if len(results) != 2 {
		return errors.New("expected two results")
	}
	host, viewer := results[0], results[1]
	if host.Role != "host" || viewer.Role != "viewer" {
		return errors.New("peer roles mismatch")
	}
	if host.Counters.LastID == 0 {
		return errors.New("no final media frame")
	}
	expected := sha256.Sum256(frame(int(host.Counters.LastID) - 1).RGBA)
	checks := []struct {
		name string
		ok   bool
	}{
		{"minimum emitted frames", host.Counters.Emitted >= 24 && viewer.Counters.Decoded > 0},
		{"final frame IDs", host.Counters.LastID == viewer.Counters.LastID && host.Counters.Emitted == uint64(host.Counters.LastID)},
		{"expected independent final hash", host.LastHash == hex.EncodeToString(expected[:]) && host.LastHash == viewer.LastHash && len(host.LastHash) == 64},
		{"aligned packet counts", host.Counters.Emitted == viewer.Counters.Received},
		{"aligned media byte counts", host.Counters.SentBytes > 0 && host.Counters.SentBytes == viewer.Counters.ReceivedBytes},
		{"media type counts", host.Counters.Keys+host.Counters.Deltas == host.Counters.Emitted && viewer.Counters.Keys+viewer.Counters.Deltas == viewer.Counters.Decoded && host.Counters.Deltas > 0 && viewer.Counters.Deltas > 0},
		{"decoder recovery", viewer.Counters.Decoded <= viewer.Counters.Received && viewer.Counters.Recoveries > 0},
		{"PLI reverse control", host.Counters.PLIAccepted > 0 && viewer.Counters.PLIRequested > 0},
		{"Ping/Pong RTT", host.Counters.Pongs > 0 && viewer.Counters.Pongs > 0 && host.Counters.RTTKnown && viewer.Counters.RTTKnown},
		{"authenticated END", viewer.Counters.Ended},
		{"worker bounds", host.GoroutinesEnd <= host.GoroutinesStart+4 && viewer.GoroutinesEnd <= viewer.GoroutinesStart+4},
		{"finite/complete duration", !math.IsNaN(host.ActiveSeconds) && !math.IsInf(host.ActiveSeconds, 0) && !math.IsNaN(viewer.ActiveSeconds) && !math.IsInf(viewer.ActiveSeconds, 0) && host.ActiveSeconds >= duration.Seconds()-0.1 && viewer.ActiveSeconds >= duration.Seconds()-0.1},
	}
	for _, check := range checks {
		if !check.ok {
			return fmt.Errorf("proof check failed: %s", check.name)
		}
	}
	return nil
}
func local(duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration+60*time.Second)
	defer cancel()
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return err
	}
	password := hex.EncodeToString(secret[:])
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	relayDone := make(chan error, 1)
	go func() { relayDone <- tcp.RunWithListener(ln, password, tcp.WithCtx(ctx), tcp.WithLogLevel("error")) }()
	defer func() { cancel(); ln.Close(); <-relayDone }()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	startChild := func(mode string, s setup) (*exec.Cmd, *json.Decoder, error) {
		cmd := exec.CommandContext(ctx, exe, "-mode", mode)
		stdin, e := cmd.StdinPipe()
		if e != nil {
			return nil, nil, e
		}
		stdout, e := cmd.StdoutPipe()
		if e != nil {
			return nil, nil, e
		}
		cmd.Stderr = io.Discard
		// With a race-enabled executable, let the default nonzero race exit code
		// propagate to Wait. Parent stdout remains sanitized result JSON only.
		if e = cmd.Start(); e != nil {
			return nil, nil, e
		}
		e = json.NewEncoder(stdin).Encode(s)
		stdin.Close()
		return cmd, json.NewDecoder(stdout), e
	}
	host, hostOutput, err := startChild("host", setup{Address: ln.Addr().String(), Password: password, Duration: duration})
	if err != nil {
		return err
	}
	defer func() {
		if host.ProcessState == nil {
			host.Process.Kill()
			host.Wait()
		}
	}()
	var invite struct{ Invite string }
	if err = hostOutput.Decode(&invite); err != nil {
		return errors.New("host setup failed")
	}
	viewer, viewerOutput, err := startChild("viewer", setup{Address: ln.Addr().String(), Password: password, Duration: duration, Invite: invite.Invite})
	invite.Invite = ""
	password = ""
	if err != nil {
		return err
	}
	defer func() {
		if viewer.ProcessState == nil {
			viewer.Process.Kill()
			viewer.Wait()
		}
	}()
	results := make([]result, 2)
	if err = hostOutput.Decode(&results[0]); err != nil {
		return errors.New("host proof failed")
	}
	if err = viewerOutput.Decode(&results[1]); err != nil {
		return errors.New("viewer proof failed")
	}
	if err = host.Wait(); err != nil {
		return errors.New("host exit failed")
	}
	if err = viewer.Wait(); err != nil {
		return errors.New("viewer exit failed")
	}
	if err = verify(results, duration); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "LOCAL_MEDIA_PASS", "results": results})
}
func main() {
	mode := flag.String("mode", "local", "local, relay, host, viewer, verify")
	duration := flag.Duration("duration", 10*time.Second, "synthetic duration")
	flag.Parse()
	var err error
	switch *mode {
	case "local":
		if *duration < 3*time.Second || *duration > 30*time.Minute {
			err = errors.New("duration out of bounds")
		} else {
			err = local(*duration)
		}
	case "relay", "host", "viewer":
		err = peer(*mode)
	case "verify":
		var results []result
		err = json.NewDecoder(io.LimitReader(os.Stdin, 65536)).Decode(&results)
		if err == nil {
			err = verify(results, *duration)
		}
		if err == nil {
			err = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "RESULTS_VERIFIED", "results": results})
		}
	default:
		err = errors.New("unknown mode")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "G4 proof failed")
		os.Exit(1)
	}
}

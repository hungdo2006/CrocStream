// g0poc is an isolated synthetic-data harness. Secrets travel over inherited
// stdin/stdout pipes during setup, never in arguments or persisted output.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
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
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"crocstream.local/g0/poc/internal/transport"
	log "github.com/schollz/croc/v11/src/logger"
	"github.com/schollz/croc/v11/src/tcp"
)

const blockSize = 32768

type config struct {
	Address      string
	Password     string
	Invite       string
	Duration     time.Duration
	SendInterval time.Duration
}
type result struct {
	Role                string  `json:"role"`
	BytesSent           uint64  `json:"bytes_sent"`
	BytesReceived       uint64  `json:"bytes_received"`
	SHA256              string  `json:"sha256_received"`
	PingsSent           uint64  `json:"pings_sent"`
	PongsReceived       uint64  `json:"pongs_received"`
	KeyRequestsReceived uint64  `json:"key_requests_received"`
	DurationSeconds     float64 `json:"duration_seconds"`
	RTTMeanMS           float64 `json:"rtt_mean_ms"`
	RTTMaxMS            float64 `json:"rtt_max_ms"`
	HeapStart           uint64  `json:"heap_start"`
	HeapEnd             uint64  `json:"heap_end"`
	GoroutinesStart     int     `json:"goroutines_start"`
	GoroutinesEnd       int     `json:"goroutines_end"`
	DataIntervalMS      int64   `json:"data_interval_ms,omitempty"`
	DataWriteMeanMS     float64 `json:"data_write_mean_ms,omitempty"`
	DataWriteMaxMS      float64 `json:"data_write_max_ms,omitempty"`
}

func main() {
	mode := flag.String("mode", "local", "local, verify, or private-stdin host/viewer/relay")
	duration := flag.Duration("duration", 10*time.Minute, "active transmission duration (1s..30m)")
	interval := flag.Duration("send-interval", 20*time.Millisecond, "local synthetic data block interval (20ms..100ms)")
	flag.Parse()
	log.SetLevel("error")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	var err error
	switch *mode {
	case "host", "viewer":
		err = peer(ctx, *mode)
	case "relay":
		err = relay(ctx)
	case "verify":
		var all []result
		decoder := json.NewDecoder(io.LimitReader(os.Stdin, 16384))
		if decodeErr := decoder.Decode(&all); decodeErr != nil {
			err = errors.New("invalid result input")
		} else if decoder.Decode(new(any)) != io.EOF {
			err = errors.New("trailing result input")
		} else if err = verifyResults(all, *duration); err == nil {
			err = json.NewEncoder(os.Stdout).Encode(struct {
				Status  string   `json:"status"`
				Results []result `json:"results"`
			}{"RESULTS_VERIFIED", all})
		}
	case "local":
		if *duration < time.Second || *duration > 30*time.Minute {
			err = errors.New("invalid duration")
		} else {
			var pacing time.Duration
			pacing, err = normalizeInterval(*interval)
			if err == nil {
				err = local(ctx, *duration, pacing)
			}
		}
	default:
		err = errors.New("invalid mode")
	}
	if err != nil {
		// Never print arbitrary peer/server errors or child pipe contents.
		fmt.Fprintln(os.Stderr, "G0_FAILED:", err)
		os.Exit(1)
	}
}

func normalizeInterval(interval time.Duration) (time.Duration, error) {
	if interval == 0 {
		return 20 * time.Millisecond, nil
	}
	if interval < 20*time.Millisecond || interval > 100*time.Millisecond ||
		interval%time.Millisecond != 0 {
		return 0, errors.New("invalid data interval")
	}
	return interval, nil
}

// relay exposes the same actual croc TCP relay used by local(), on one
// explicitly configured port. G0 does not need stock file-transfer port routing.
func relay(parent context.Context) error {
	var cfg config
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 4096)).Decode(&cfg); err != nil {
		return errors.New("invalid private relay setup")
	}
	if cfg.Duration < time.Second || cfg.Duration > 30*time.Minute ||
		cfg.Password == "" || len(cfg.Password) > 1024 {
		return errors.New("invalid relay duration or password")
	}
	ctx, cancel := context.WithTimeout(parent, cfg.Duration+90*time.Second)
	defer cancel()
	ln, err := net.Listen("tcp4", cfg.Address)
	if err != nil {
		return errors.New("relay listen failed")
	}
	defer ln.Close()
	if err := json.NewEncoder(os.Stdout).Encode(struct {
		Event string `json:"event"`
	}{"relay_ready"}); err != nil {
		return errors.New("relay readiness output failed")
	}
	err = tcp.RunWithListener(ln, cfg.Password, tcp.WithCtx(ctx), tcp.WithLogLevel("error"))
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return errors.New("relay stopped unexpectedly")
	}
	return nil
}

func peer(ctx context.Context, role string) error {
	var cfg config
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 4096)).Decode(&cfg); err != nil {
		return errors.New("invalid private setup")
	}
	if cfg.Duration < time.Second || cfg.Duration > 30*time.Minute {
		return errors.New("invalid duration")
	}
	interval, err := normalizeInterval(cfg.SendInterval)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Duration+45*time.Second)
	defer cancel()
	opts := transport.Options{RelayAddress: cfg.Address, RelayPassword: cfg.Password}
	var s *transport.Session
	if role == "host" {
		l, err := transport.Host(ctx, opts)
		if err != nil {
			return errors.New("host admission failed")
		}
		defer l.Close()
		// Parent reads this single private setup message, never logs it.
		if err := json.NewEncoder(os.Stdout).Encode(struct{ Invite string }{l.Invitation()}); err != nil {
			return errors.New("private setup failed")
		}
		s, err = l.Accept(ctx)
		if err != nil {
			return errors.New("host authentication failed")
		}
	} else {
		var err error
		s, err = transport.Join(ctx, cfg.Invite, opts)
		if err != nil {
			return errors.New("viewer authentication failed")
		}
	}
	defer s.Close()
	cfg.Password, cfg.Invite = "", ""
	r, err := exchange(ctx, s, role, cfg.Duration, interval)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(r)
}

// Harness messages are separate from both transport records and G1 media.
// Header: kind[1], sequence/nonce[8 BE], payload length[4 BE].
type message struct {
	kind byte
	id   uint64
	data []byte
}

func writeMessage(w io.Writer, m message) error {
	if len(m.data) > blockSize {
		return errors.New("harness message too large")
	}
	b := make([]byte, 13+len(m.data))
	b[0] = m.kind
	binary.BigEndian.PutUint64(b[1:9], m.id)
	binary.BigEndian.PutUint32(b[9:13], uint32(len(m.data)))
	copy(b[13:], m.data)
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}
func readMessage(r io.Reader) (message, error) {
	var h [13]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return message{}, err
	}
	n := binary.BigEndian.Uint32(h[9:13])
	switch h[0] {
	case 1:
		if n != blockSize {
			return message{}, errors.New("bad data size")
		}
	case 2, 3, 4, 5, 6:
		if n != 0 {
			return message{}, errors.New("bad control size")
		}
	default:
		return message{}, errors.New("bad kind")
	}
	b := make([]byte, int(n))
	_, err := io.ReadFull(r, b)
	return message{h[0], binary.BigEndian.Uint64(h[1:9]), b}, err
}
func fill(b []byte, seq uint64, role byte) {
	// Cheap deterministic nonconstant pattern, distinct for every role/sequence.
	x := seq ^ uint64(role+1)*0x9e3779b97f4a7c15
	for i := range b {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		b[i] = byte(x)
	}
}

func exchange(ctx context.Context, s *transport.Session, role string, duration, interval time.Duration) (result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	localRole := byte(0)
	if role == "host" {
		localRole = 1
	}
	start := time.Now()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	r := result{Role: role, HeapStart: mem.HeapAlloc, GoroutinesStart: runtime.NumGoroutine(),
		DataIntervalMS: interval.Milliseconds()}
	controls := make(chan message, 8)
	received := make(chan error, 1)
	var wg sync.WaitGroup
	var mu sync.Mutex // counters updated by reader, sampled after its completion
	var rx, keyExpected, pingExpected uint64
	var finalTX atomic.Uint64
	var rttSum, rttMax time.Duration
	hashRX := sha256.New()
	wg.Add(1)
	go func() {
		defer wg.Done()
		fail := func(err error) { received <- err }
		peerDone, gotAck := false, false
		for {
			m, err := readMessage(s)
			if err != nil {
				fail(errors.New("harness receive failed"))
				return
			}
			switch m.kind {
			case 1:
				if peerDone {
					fail(errors.New("data after done"))
					return
				}
				if m.id != rx {
					fail(errors.New("data ordering failed"))
					return
				}
				want := make([]byte, blockSize)
				fill(want, rx, localRole^1)
				if !equal(want, m.data) {
					fail(errors.New("data corruption"))
					return
				}
				_, _ = hashRX.Write(m.data)
				rx++
			case 2: // ping from peer; echo through the ONE writer
				if m.id != pingExpected {
					fail(errors.New("ping ordering failed"))
					return
				}
				pingExpected++
				select {
				case controls <- message{kind: 3, id: m.id}:
				case <-ctx.Done():
					fail(ctx.Err())
					return
				}
			case 3:
				mu.Lock()
				if m.id != r.PongsReceived {
					mu.Unlock()
					fail(errors.New("pong ordering failed"))
					return
				}
				// Actual send times are kept in a single-outstanding slot.
				sent, ok := pingTimes.LoadAndDelete(pingKey{s, m.id})
				if !ok {
					mu.Unlock()
					fail(errors.New("unexpected pong"))
					return
				}
				elapsed := time.Since(sent.(time.Time))
				rttSum += elapsed
				if elapsed > rttMax {
					rttMax = elapsed
				}
				r.PongsReceived++
				mu.Unlock()
			case 4:
				if m.id != keyExpected {
					fail(errors.New("key request ordering failed"))
					return
				}
				keyExpected++
			case 5: // peer has sent all data/pings; ACK after its data was verified
				if peerDone || m.id != rx || rx < 320 {
					fail(errors.New("peer byte count failed"))
					return
				}
				peerDone = true
				select {
				case controls <- message{kind: 6, id: rx}:
				case <-ctx.Done():
					fail(ctx.Err())
					return
				}
			case 6:
				if gotAck || finalTX.Load() == 0 || m.id != finalTX.Load() {
					fail(errors.New("invalid acknowledgement"))
					return
				}
				gotAck = true
			}
			if peerDone && gotAck {
				fail(nil)
				return
			}
		}
	}()
	finish := func(err error) (result, error) {
		cancel()
		_ = s.Close()
		wg.Wait()
		pingTimes.Range(func(k, v any) bool {
			if k.(pingKey).s == s {
				pingTimes.Delete(k)
			}
			return true
		})
		return r, err
	}
	var tx uint64
	var pings uint64
	var writeTotal, writeMax time.Duration
	buf := make([]byte, blockSize)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	doneSent := false
	ackSent := false
	readerDone := false
	// Only one ping outstanding; one data record per tick. 320 blocks = 10MiB.
	for !(readerDone && ackSent) {
		// Priority for queued control; no concurrent writes into the stream.
		select {
		case c := <-controls:
			if err := writeMessage(s, c); err != nil {
				return finish(errors.New("control write failed"))
			}
			if c.kind == 6 {
				ackSent = true
			}
			continue
		default:
		}
		select {
		case <-ctx.Done():
			return finish(errors.New("exchange canceled or timed out"))
		case err := <-received:
			if err != nil {
				return finish(err)
			}
			readerDone = true
		case c := <-controls:
			if err := writeMessage(s, c); err != nil {
				return finish(errors.New("control write failed"))
			}
			if c.kind == 6 {
				ackSent = true
			}
		case <-ticker.C:
			if doneSent {
				continue
			}
			mu.Lock()
			outstanding := pings - r.PongsReceived
			mu.Unlock()
			if time.Since(start) >= duration && tx >= 320 && outstanding == 0 {
				finalTX.Store(tx)
				if err := writeMessage(s, message{kind: 5, id: tx}); err != nil {
					return finish(errors.New("done write failed"))
				}
				doneSent = true
				continue
			}
			fill(buf, tx, localRole)
			writeStart := time.Now()
			if err := writeMessage(s, message{kind: 1, id: tx, data: buf}); err != nil {
				return finish(errors.New("data write failed"))
			}
			writeElapsed := time.Since(writeStart)
			writeTotal += writeElapsed
			if writeElapsed > writeMax {
				writeMax = writeElapsed
			}
			tx++
			if outstanding == 0 {
				pingTimes.Store(pingKey{s, pings}, time.Now())
				if err := writeMessage(s, message{kind: 2, id: pings}); err != nil {
					return finish(errors.New("ping write failed"))
				}
				if err := writeMessage(s, message{kind: 4, id: pings}); err != nil {
					return finish(errors.New("key request write failed"))
				}
				pings++
			}
		}
	}
	wg.Wait()
	r.BytesSent = tx * blockSize
	r.BytesReceived = rx * blockSize
	r.PingsSent = pings
	r.KeyRequestsReceived = keyExpected
	r.DurationSeconds = time.Since(start).Seconds()
	r.SHA256 = hex.EncodeToString(hashRX.Sum(nil))
	r.DataWriteMeanMS = float64(writeTotal) / float64(time.Millisecond) / float64(tx)
	r.DataWriteMaxMS = float64(writeMax) / float64(time.Millisecond)
	r.RTTMaxMS = float64(rttMax) / float64(time.Millisecond)
	if r.PongsReceived != pings || pings == 0 {
		return finish(errors.New("missing pong"))
	}
	r.RTTMeanMS = float64(rttSum) / float64(time.Millisecond) / float64(pings)
	runtime.ReadMemStats(&mem)
	r.HeapEnd = mem.HeapAlloc
	r.GoroutinesEnd = runtime.NumGoroutine()
	return r, nil
}

type pingKey struct {
	s  *transport.Session
	id uint64
}

var pingTimes sync.Map // bounded to one outstanding ping per process
func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type child struct {
	cmd      *exec.Cmd
	input    io.WriteCloser
	output   *json.Decoder
	waitOnce sync.Once
	waitErr  error
}

func (c *child) wait() error {
	c.waitOnce.Do(func() { c.waitErr = c.cmd.Wait() })
	return c.waitErr
}

func (c *child) cleanup() {
	_ = c.input.Close()
	_ = c.cmd.Process.Kill()
	_ = c.wait()
}

func startChild(ctx context.Context, role string) (*child, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, exe, "-mode", role)
	cmd.WaitDelay = 3 * time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	cmd.Stderr = io.Discard // child errors bounded; don't forward arbitrary upstream output
	if err := cmd.Start(); err != nil {
		in.Close()
		out.Close()
		return nil, err
	}
	// Wait only after decoder drains stdout (StdoutPipe contract).
	return &child{cmd: cmd, input: in, output: json.NewDecoder(bufio.NewReader(out))}, nil
}
func configure(c *child, cfg config) error {
	err := json.NewEncoder(c.input).Encode(cfg)
	_ = c.input.Close()
	return err
}
func collect(c *child) (result, error) {
	var r result
	if err := c.output.Decode(&r); err != nil {
		_ = c.cmd.Process.Kill()
		_ = c.wait()
		return r, errors.New("child result missing")
	}
	if err := c.wait(); err != nil {
		return r, errors.New("child failed")
	}
	return r, nil
}
func local(parent context.Context, d, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(parent, d+60*time.Second)
	defer cancel()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		ln.Close()
		return err
	}
	password := hex.EncodeToString(passwordBytes)
	clear(passwordBytes)
	relayDone := make(chan error, 1)
	go func() { relayDone <- tcp.RunWithListener(ln, password, tcp.WithCtx(ctx), tcp.WithLogLevel("error")) }()
	defer func() { cancel(); _ = ln.Close(); <-relayDone }()
	host, err := startChild(ctx, "host")
	if err != nil {
		return errors.New("host process start failed")
	}
	defer host.cleanup()
	cfg := config{Address: ln.Addr().String(), Password: password, Duration: d, SendInterval: interval}
	if err := configure(host, cfg); err != nil {
		return errors.New("host setup failed")
	}
	var invitation struct{ Invite string }
	if err := host.output.Decode(&invitation); err != nil {
		_ = host.wait()
		return errors.New("host invitation missing")
	}
	viewer, err := startChild(ctx, "viewer")
	if err != nil {
		return errors.New("viewer process start failed")
	}
	defer viewer.cleanup()
	cfg.Invite = invitation.Invite
	invitation.Invite = ""
	if err := configure(viewer, cfg); err != nil {
		return errors.New("viewer setup failed")
	}
	cfg.Password, cfg.Invite = "", ""
	fmt.Println(`{"event":"started","route":"loopback-croc-relay","peer_processes":2}`)
	type outcome struct {
		r   result
		err error
	}
	results := make(chan outcome, 2)
	for _, c := range []*child{host, viewer} {
		go func(c *child) { r, e := collect(c); results <- outcome{r, e} }(c)
	}
	var all []result
	var firstErr error
	for range 2 {
		out := <-results
		if out.err != nil {
			cancel()
			if firstErr == nil {
				firstErr = out.err
			}
		}
		all = append(all, out.r)
	}
	if firstErr != nil {
		return firstErr
	}
	if err := verifyResults(all, d); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Status  string   `json:"status"`
		Results []result `json:"results"`
	}{"LOCAL_PASS", all})
}

// verifyResults also accepts separately collected remote peer results. It proves
// counters/content, not the network route or the authenticity of supplied files.
func verifyResults(all []result, d time.Duration) error {
	if d < time.Second || d > 30*time.Minute || len(all) != 2 ||
		!((all[0].Role == "host" && all[1].Role == "viewer") ||
			(all[0].Role == "viewer" && all[1].Role == "host")) {
		return errors.New("invalid result roles or duration")
	}
	for _, r := range all {
		other := all[0]
		if other.Role == r.Role {
			other = all[1]
		}
		// Older evidence did not contain pacing/write-timing fields.
		if r.DataIntervalMS != 0 && (r.DataIntervalMS < 20 || r.DataIntervalMS > 100) {
			return errors.New("invalid result data interval")
		}
		for _, timing := range []float64{r.DataWriteMeanMS, r.DataWriteMaxMS} {
			if timing < 0 || math.IsNaN(timing) || math.IsInf(timing, 0) {
				return errors.New("invalid result write timing")
			}
		}
		// Bound independent hash work even for untrusted result JSON.
		if r.BytesReceived < 320*blockSize || r.BytesReceived > 1<<32 ||
			r.BytesReceived%blockSize != 0 ||
			r.BytesSent < 320*blockSize || r.BytesSent > 1<<32 ||
			r.BytesSent%blockSize != 0 ||
			r.PingsSent == 0 || r.PingsSent > r.BytesSent/blockSize ||
			r.PingsSent != r.PongsReceived ||
			math.IsNaN(r.DurationSeconds) || math.IsInf(r.DurationSeconds, 0) ||
			r.DurationSeconds > (d+45*time.Second).Seconds() {
			return errors.New("invalid result bounds or pong count")
		}
		if r.BytesReceived != other.BytesSent || r.KeyRequestsReceived != other.PingsSent ||
			r.DurationSeconds < d.Seconds() {
			return errors.New("cross-process counters failed")
		}
		// Independently compute expected hash from sender role and block count.
		h := sha256.New()
		b := make([]byte, blockSize)
		sender := byte(0)
		if other.Role == "host" {
			sender = 1
		}
		for i := uint64(0); i < r.BytesReceived/blockSize; i++ {
			fill(b, i, sender)
			_, _ = h.Write(b)
		}
		if hex.EncodeToString(h.Sum(nil)) != r.SHA256 {
			return errors.New("cross-process hash failed")
		}
	}
	return nil
}

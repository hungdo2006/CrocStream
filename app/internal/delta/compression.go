package delta

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"

	"crocstream.local/app/internal/protocol"
	"github.com/pierrec/lz4/v4"
)

var ErrCompressed = errors.New("delta: invalid or oversized LZ4 frame")

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > protocol.MaxPayload-b.Len() {
		return 0, ErrCompressed
	}
	return b.Buffer.Write(p)
}

// Every invocation emits a fresh standard LZ4 frame, deterministic settings,
// independent 64-KiB blocks, content size, block and content checksums.
func compress(ctx context.Context, raw []byte) ([]byte, error) {
	var out boundedBuffer
	w := lz4.NewWriter(&out)
	defer w.Reset(io.Discard) // Release library buffers on success or failure.
	return compressInto(ctx, w, &out, raw)
}

func compressInto(ctx context.Context, w *lz4.Writer, out *boundedBuffer, raw []byte) ([]byte, error) {
	if len(raw) > protocol.MaxRaw+2+2*920 {
		return nil, ErrCompressed
	}
	out.Reset()
	w.Reset(out)
	if err := w.Apply(lz4.ConcurrencyOption(1), lz4.BlockSizeOption(lz4.Block64Kb),
		lz4.CompressionLevelOption(lz4.Fast), lz4.SizeOption(uint64(len(raw))),
		lz4.ChecksumOption(true), lz4.BlockChecksumOption(true)); err != nil {
		return nil, err
	}
	for rest := raw; len(rest) > 0; {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := min(len(rest), 64*1024)
		if _, err := w.Write(rest[:n]); err != nil {
			return nil, ErrCompressed
		}
		rest = rest[n:]
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, ErrCompressed
	}
	return out.Bytes(), nil
}

// singleFrame checks the envelope without decompression/allocation. The library
// supports concatenated/legacy/skippable frames; media v1 permits one standard,
// self-contained frame only. Dictionaries require out-of-band state and are
// rejected. Header/block/content checksums remain the library's responsibility.
// Both independent and dependent blocks within this one frame are supported.
func singleFrame(ctx context.Context, b []byte, limit int) error {
	if limit < 0 || limit > protocol.MaxRaw+2+2*920 ||
		len(b) < 11 || len(b) > protocol.MaxPayload ||
		binary.LittleEndian.Uint32(b[:4]) != 0x184D2204 {
		return ErrCompressed
	}
	flg, bd := b[4], b[5]
	code := (bd >> 4) & 7
	if flg>>6 != 1 || flg&3 != 0 || bd&0x8F != 0 || code < 4 || code > 7 {
		return ErrCompressed
	}
	blockMax := 1 << (16 + 2*(code-4))
	pos := 6
	if flg&8 != 0 {
		if len(b)-pos < 9 || binary.LittleEndian.Uint64(b[pos:pos+8]) > uint64(limit) {
			return ErrCompressed
		}
		pos += 8
	}
	pos++ // Header checksum; validated by lz4.Reader.
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(b)-pos < 4 {
			return ErrCompressed
		}
		word := binary.LittleEndian.Uint32(b[pos : pos+4])
		pos += 4
		if word == 0 {
			if flg&4 != 0 {
				pos += 4
			}
			if pos != len(b) {
				return ErrCompressed
			}
			return nil
		}
		n := int(word & 0x7FFFFFFF)
		if n == 0 || n > blockMax || n > len(b)-pos {
			return ErrCompressed
		}
		pos += n
		if flg&16 != 0 {
			if len(b)-pos < 4 {
				return ErrCompressed
			}
			pos += 4
		}
	}
}

// decompress reads at most limit+1 bytes. The additional byte detects overshoot;
// fixed library block buffers are <=4 MiB, with concurrency=1 and no worker
// goroutines. EOF is required so checksums and declared size are always checked.
func decompress(ctx context.Context, payload []byte, limit int) ([]byte, error) {
	r := lz4.NewReader(nil)
	defer r.Reset(nil)
	return decompressInto(ctx, r, nil, payload, limit)
}

func decompressInto(ctx context.Context, r *lz4.Reader, dst []byte, payload []byte, limit int) ([]byte, error) {
	if err := singleFrame(ctx, payload, limit); err != nil {
		return nil, err
	}
	r.Reset(bytes.NewReader(payload))
	if err := r.Apply(lz4.ConcurrencyOption(1)); err != nil {
		return nil, err
	}
	need := limit + 1
	if cap(dst) < need {
		dst = make([]byte, need)
	} else {
		dst = dst[:need]
	}
	used := 0
	for used < len(dst) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := r.Read(dst[used:min(used+64*1024, len(dst))])
		used += n
		if used > limit {
			return nil, ErrCompressed
		}
		if err == io.EOF {
			return dst[:used], nil
		}
		if err != nil || n == 0 {
			return nil, ErrCompressed
		}
	}
	return nil, ErrCompressed
}

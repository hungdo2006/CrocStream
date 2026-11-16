// Package protocol implements CrocStream media protocol v1 framing and CONFIG.
// It runs inside an already authenticated, encrypted, reliable duplex transport.
//
// Reader and Writer each have one owner and are not safe for concurrent use.
// The session must serialize outgoing controls/video and close the underlying
// transport (or set deadlines) to cancel blocked I/O. This package starts no
// goroutines, owns no connection, and retains no payload buffers.
//
// Video payloads are opaque here: the G2 codec must validate bounded LZ4-frame
// decompression and the entire image/tile record before Reader.ResolveVideo(true).
// Framing success alone never establishes a valid canonical image.
package protocol

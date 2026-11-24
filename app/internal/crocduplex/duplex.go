// Package duplex exposes the verified G0 transport without copying its crypto
// or weakening Go internal-package boundaries. The experimental transport
// purpose, record format and limits remain exactly those tested at G0.
package crocduplex

import (
	"context"
	"io"

	"crocstream.local/app/internal/crocduplex/transport"
)

type Options = transport.Options
type Listener interface {
	Invitation() string
	Accept(context.Context) (io.ReadWriteCloser, error)
	Close() error
}
type listener struct{ inner *transport.Listener }

func Host(ctx context.Context, options Options) (Listener, error) {
	l, err := transport.Host(ctx, options)
	if err != nil {
		return nil, err
	}
	return &listener{l}, nil
}
func (l *listener) Invitation() string { return l.inner.Invitation() }
func (l *listener) Accept(ctx context.Context) (io.ReadWriteCloser, error) {
	stream, err := l.inner.Accept(ctx)
	if err != nil {
		return nil, err
	}
	return stream, nil
}
func (l *listener) AcceptWithProgress(ctx context.Context, authenticating func()) (io.ReadWriteCloser, error) {
	stream, err := l.inner.AcceptWithProgress(ctx, authenticating)
	if err != nil {
		return nil, err
	}
	return stream, nil
}
func (l *listener) Close() error { return l.inner.Close() }
func Join(ctx context.Context, invitation string, options Options) (io.ReadWriteCloser, error) {
	return JoinWithProgress(ctx, invitation, options, nil)
}
func JoinWithProgress(ctx context.Context, invitation string, options Options, authenticating func()) (io.ReadWriteCloser, error) {
	stream, err := transport.JoinWithProgress(ctx, invitation, options, authenticating)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

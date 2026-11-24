// Package transport is the application's io.ReadWriteCloser transport boundary.
// All cryptographic implementation stays in the verified G0 module.
package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"time"

	"crocstream.local/app/internal/crocduplex"
)

type Options struct {
	Address  string `json:"address"`
	Password string `json:"password"`
}

func (o Options) Validate() error {
	host, port, err := net.SplitHostPort(o.Address)
	n, e := strconv.Atoi(port)
	if err != nil || e != nil || host == "" || len(o.Address) > 300 || n < 1 || n > 65535 || o.Password == "" || len(o.Password) > 1024 {
		return errors.New("Cần địa chỉ relay host:port và mật khẩu relay hợp lệ.")
	}
	return nil
}

type Listener = crocduplex.Listener
type Factory interface {
	Host(context.Context, Options, func()) (Listener, error)
	Join(context.Context, string, Options, func()) (io.ReadWriteCloser, error)
}
type Croc struct{}

func options(o Options) crocduplex.Options {
	return crocduplex.Options{RelayAddress: o.Address, RelayPassword: o.Password, IOTimeout: 30 * time.Second}
}
func (Croc) Host(ctx context.Context, o Options, authenticating func()) (Listener, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	l, err := crocduplex.Host(ctx, options(o))
	if err != nil {
		return nil, err
	}
	return &listenerProgress{Listener: l, progress: authenticating}, nil
}
func (Croc) Join(ctx context.Context, code string, o Options, authenticating func()) (io.ReadWriteCloser, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return crocduplex.JoinWithProgress(ctx, code, options(o), authenticating)
}

type listenerProgress struct {
	Listener
	progress func()
}

func (l *listenerProgress) Accept(ctx context.Context) (io.ReadWriteCloser, error) {
	if p, ok := l.Listener.(interface {
		AcceptWithProgress(context.Context, func()) (io.ReadWriteCloser, error)
	}); ok {
		return p.AcceptWithProgress(ctx, l.progress)
	}
	return l.Listener.Accept(ctx)
}

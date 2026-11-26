package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

const maxRelayConfigBytes = 4096

// LoadRelayFile reads an optional JSON relay config file with bounded size and
// strict schema validation. A missing file returns empty Options and nil error.
func LoadRelayFile(path string) (Options, error) {
	if strings.TrimSpace(path) == "" {
		return Options{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Options{}, nil
		}
		return Options{}, errors.New("cannot open relay config")
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxRelayConfigBytes+1))
	if err != nil {
		return Options{}, errors.New("cannot read relay config")
	}
	if len(raw) > maxRelayConfigBytes {
		return Options{}, errors.New("relay config exceeds maximum size")
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	var opts Options
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opts); err != nil {
		return Options{}, errors.New("invalid relay config json")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Options{}, errors.New("unexpected trailing data in relay config")
	}
	opts.Address = strings.TrimSpace(opts.Address)
	if err := opts.Validate(); err != nil {
		return Options{}, err
	}
	return opts, nil
}

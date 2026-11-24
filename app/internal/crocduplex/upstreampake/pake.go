// Package upstreampake is the smallest CrocStream-derived copy of croc's
// transcript-bound PAKE key schedule. It is intentionally isolated from the
// pristine croc snapshot and adds a separate purpose namespace.
package upstreampake

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/schollz/pake/v3"
	"golang.org/x/crypto/hkdf"
)

const (
	ProtocolVersion = 2
	SaltSize        = 32
	Purpose         = "peer-crocstream-g0-v1"
	Curve           = "p256"

	identityDomain   = "croc/spake2/participant/v2"
	transcriptDomain = "croc/spake2/channel/v2"
)

type Context struct {
	Initiator []byte
	Responder []byte
	Salt      []byte
	Room      string
}

type Keys struct {
	EncryptionKey []byte
	ConfirmationA []byte
	ConfirmationB []byte
}

func Init(password []byte, role int, room string) (*pake.Pake, error) {
	idA, idB, err := identities(room)
	if err != nil {
		return nil, err
	}
	return pake.InitCurveWithIdentities(password, role, Curve, idA, idB)
}

func Derive(sharedKey []byte, context Context) (Keys, error) {
	if len(sharedKey) == 0 {
		return Keys{}, fmt.Errorf("PAKE session key is required")
	}
	if context.Room == "" {
		return Keys{}, fmt.Errorf("PAKE room is required")
	}
	if len(context.Initiator) == 0 || len(context.Responder) == 0 {
		return Keys{}, fmt.Errorf("both PAKE wire values are required")
	}
	if len(context.Salt) != SaltSize {
		return Keys{}, fmt.Errorf("PAKE salt must be %d bytes", SaltSize)
	}
	idA, idB, err := identities(context.Room)
	if err != nil {
		return Keys{}, err
	}
	var version [8]byte
	binary.LittleEndian.PutUint64(version[:], ProtocolVersion)
	transcript := frame(
		[]byte(transcriptDomain),
		version[:],
		[]byte(Purpose),
		[]byte(context.Room),
		[]byte(Curve),
		idA,
		idB,
		context.Initiator,
		context.Responder,
		context.Salt,
	)
	material := make([]byte, 96)
	defer clear(material)
	if _, err := io.ReadFull(hkdf.New(sha256.New, sharedKey, context.Salt, transcript), material); err != nil {
		return Keys{}, fmt.Errorf("derive PAKE channel keys: %w", err)
	}
	return Keys{
		EncryptionKey: append([]byte(nil), material[:32]...),
		ConfirmationA: confirmationTag(material[32:64], transcript),
		ConfirmationB: confirmationTag(material[64:96], transcript),
	}, nil
}

func Confirm(expected, received []byte) bool {
	return len(expected) == sha256.Size && hmac.Equal(expected, received)
}

func identities(room string) (idA, idB []byte, err error) {
	if room == "" {
		return nil, nil, fmt.Errorf("PAKE room is required")
	}
	var version [8]byte
	binary.LittleEndian.PutUint64(version[:], ProtocolVersion)
	return frame([]byte(identityDomain), version[:], []byte(Purpose), []byte(room), []byte("receiver")),
		frame([]byte(identityDomain), version[:], []byte(Purpose), []byte(room), []byte("sender")), nil
}

func confirmationTag(key, transcript []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(transcript)
	return mac.Sum(nil)
}

func frame(fields ...[]byte) []byte {
	total := 0
	for _, field := range fields {
		total += 8 + len(field)
	}
	out := make([]byte, 0, total)
	var length [8]byte
	for _, field := range fields {
		binary.LittleEndian.PutUint64(length[:], uint64(len(field)))
		out = append(out, length[:]...)
		out = append(out, field...)
	}
	return out
}

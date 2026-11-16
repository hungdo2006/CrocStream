package upstreampake

import (
	"bytes"
	"testing"

	upstream "github.com/schollz/croc/v11/src/pakekey"
)

func TestAgreementAndIsolation(t *testing.T) {
	for _, mode := range []string{"valid", "wrong-password", "wrong-room", upstream.PurposeTransfer, upstream.PurposeLocalProbe, upstream.PurposeSSH} {
		t.Run(mode, func(t *testing.T) {
			a, err := Init([]byte("test password"), 0, "room")
			if err != nil {
				t.Fatal(err)
			}
			pw, room := "test password", "room"
			if mode == "wrong-password" {
				pw = "other password"
			}
			if mode == "wrong-room" {
				room = "other room"
			}
			b, err := Init([]byte(pw), 1, room)
			isUpstream := mode == upstream.PurposeTransfer || mode == upstream.PurposeLocalProbe || mode == upstream.PurposeSSH
			if isUpstream {
				b, err = upstream.Init([]byte(pw), 1, Curve, mode, room)
			}
			if err != nil {
				t.Fatal(err)
			}
			aBytes := a.Bytes()
			if err := b.Update(aBytes); err != nil {
				if mode == "valid" {
					t.Fatal(err)
				}
				return
			}
			bBytes := b.Bytes()
			if err := a.Update(bBytes); err != nil {
				if mode == "valid" {
					t.Fatal(err)
				}
				return
			}
			ka, err := a.SessionKey()
			if err != nil {
				if mode == "valid" {
					t.Fatal(err)
				}
				return
			}
			kb, err := b.SessionKey()
			if err != nil {
				if mode == "valid" {
					t.Fatal(err)
				}
				return
			}
			c := Context{Room: "room", Initiator: aBytes, Responder: bBytes, Salt: bytes.Repeat([]byte{2}, 32)}
			keysA, err := Derive(ka, c)
			if err != nil {
				t.Fatal(err)
			}
			c.Room = room
			keysB, err := Derive(kb, c)
			if err != nil {
				t.Fatal(err)
			}
			if isUpstream {
				up, err := upstream.Derive(kb, upstream.Context{Purpose: mode, Room: room, Curve: Curve, Initiator: aBytes, Responder: bBytes, Salt: c.Salt})
				if err != nil {
					t.Fatal(err)
				}
				keysB = Keys{up.EncryptionKey, up.ConfirmationA, up.ConfirmationB}
			}
			agrees := Confirm(keysA.ConfirmationA, keysB.ConfirmationA) && Confirm(keysA.ConfirmationB, keysB.ConfirmationB) && bytes.Equal(keysA.EncryptionKey, keysB.EncryptionKey)
			if agrees != (mode == "valid") {
				t.Fatal("unexpected agreement")
			}
			if Confirm(keysA.ConfirmationA, keysA.ConfirmationB) {
				t.Fatal("role confirmation reflection")
			}
		})
	}
}

func TestTranscriptAndValidation(t *testing.T) {
	c := Context{Room: "room", Initiator: []byte("a"), Responder: []byte("b"), Salt: make([]byte, 32)}
	want, err := Derive([]byte("key"), c)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"room", "a", "b", "salt"} {
		d := c
		switch field {
		case "room":
			d.Room = "other"
		case "a":
			d.Initiator = []byte("c")
		case "b":
			d.Responder = []byte("d")
		case "salt":
			d.Salt = bytes.Repeat([]byte{1}, 32)
		}
		got, err := Derive([]byte("key"), d)
		if err != nil {
			t.Fatal(err)
		}
		if Confirm(want.ConfirmationA, got.ConfirmationA) {
			t.Fatal("transcript not bound")
		}
	}
	if _, err := Derive(nil, c); err == nil {
		t.Fatal("missing key")
	}
	c.Salt = nil
	if _, err := Derive([]byte("key"), c); err == nil {
		t.Fatal("missing salt")
	}
	if _, err := Init([]byte("p"), 0, ""); err == nil {
		t.Fatal("empty room")
	}
}

func TestUpstreamInitiatorIsolation(t *testing.T) {
	for _, purpose := range []string{upstream.PurposeTransfer, upstream.PurposeLocalProbe, upstream.PurposeSSH} {
		t.Run(purpose, func(t *testing.T) {
			a, err := upstream.Init([]byte("test"), 0, Curve, purpose, "room")
			if err != nil {
				t.Fatal(err)
			}
			b, err := Init([]byte("test"), 1, "room")
			if err != nil {
				t.Fatal(err)
			}
			av := a.Bytes()
			if err := b.Update(av); err != nil {
				return
			}
			bv := b.Bytes()
			if err := a.Update(bv); err != nil {
				return
			}
			ak, err := a.SessionKey()
			if err != nil {
				return
			}
			bk, err := b.SessionKey()
			if err != nil {
				return
			}
			salt := make([]byte, 32)
			old, err := upstream.Derive(ak, upstream.Context{Purpose: purpose, Room: "room", Curve: Curve, Initiator: av, Responder: bv, Salt: salt})
			if err != nil {
				t.Fatal(err)
			}
			newKeys, err := Derive(bk, Context{Room: "room", Initiator: av, Responder: bv, Salt: salt})
			if err != nil {
				t.Fatal(err)
			}
			if Confirm(old.ConfirmationA, newKeys.ConfirmationA) || Confirm(old.ConfirmationB, newKeys.ConfirmationB) {
				t.Fatal("old initiator authenticated")
			}
		})
	}
}

func TestMalformedPAKEAndSameRole(t *testing.T) {
	for _, bad := range [][]byte{[]byte("null"), []byte("{}"), []byte("[]"), []byte("invalid")} {
		p, err := Init([]byte("test"), 1, "room")
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Update(bad); err == nil {
			t.Fatal("malformed PAKE accepted")
		}
	}
	a, _ := Init([]byte("test"), 0, "room")
	b, _ := Init([]byte("test"), 0, "room")
	if err := a.Update(b.Bytes()); err == nil {
		t.Fatal("same role accepted")
	}
}

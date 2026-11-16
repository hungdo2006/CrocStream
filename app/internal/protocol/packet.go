package protocol

// Packet owns no hidden storage. DecodePacket borrows the input slice;
// Reader.ReadPacket returns a newly allocated payload owned by the caller.
// Writer.WritePacket borrows Payload only for the duration of the call.
type Packet struct {
	Header  Header
	Payload []byte
}

func (p Packet) Validate(d Direction) error {
	if err := p.Header.Validate(d); err != nil {
		return err
	}
	if uint64(len(p.Payload)) != uint64(p.Header.PayloadLength) {
		return ErrLength
	}
	if p.Header.Type == SessionConfig {
		_, err := DecodeConfig(p.Payload)
		return err
	}
	return nil
}

// DecodePacket decodes exactly one packet without allocating/copying payload.
// Stream ordering requires Reader/Writer; video decompression requires the codec.
func DecodePacket(b []byte, d Direction) (Packet, error) {
	if len(b) < HeaderSize {
		return Packet{}, ErrLength
	}
	h, err := DecodeHeader(b[:HeaderSize])
	if err != nil {
		return Packet{}, err
	}
	p := Packet{Header: h, Payload: b[HeaderSize:]}
	if err := p.Validate(d); err != nil {
		return Packet{}, err
	}
	return p, nil
}

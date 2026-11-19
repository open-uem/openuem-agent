package runtime

import "bytes"

// MaxCommandOutput limits the output kept of the commands the agent runs, e.g. winget in
// a user's session, whose binary the user controls
const MaxCommandOutput = 4 << 20

// CappedBuffer keeps the first Max bytes written to it and drops the rest. It keeps
// accepting writes, so the command writing to it isn't blocked.
type CappedBuffer struct {
	Max       int
	buf       bytes.Buffer
	truncated bool
}

func (b *CappedBuffer) Write(p []byte) (int, error) {
	if room := b.Max - b.buf.Len(); room < len(p) {
		if room > 0 {
			b.buf.Write(p[:room])
		}
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *CappedBuffer) Bytes() []byte {
	if b.truncated {
		return append(bytes.Clone(b.buf.Bytes()), []byte("\n[output truncated]")...)
	}
	return b.buf.Bytes()
}

func (b *CappedBuffer) String() string { return string(b.Bytes()) }

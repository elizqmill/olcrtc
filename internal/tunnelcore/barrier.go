package tunnelcore

import (
	"bytes"
	"time"

	"github.com/openlibrecommunity/olcrtc/internal/muxconn"
)

// BarrierTimeout bounds how long a side waits for the peer barrier. On
// timeout the side proceeds without barrier confirmation, preserving
// interop with peers that predate the barrier protocol.
const BarrierTimeout = 2 * time.Second

// barrierMaxFrames caps how many stale frames a drain consumes before
// giving up, so a frame flood cannot pin the pre-smux window forever.
const barrierMaxFrames = 32

// barrierFrame marks the start of a fresh session generation. On transports
// whose underlying channel survives session teardown (shared rooms, buffered
// bridges), frames of the previous generation stay in flight and would
// otherwise be fed to the freshly installed smux session: a stale SYN
// carrying a CONNECT request lands where the handshake expects CLIENT_HELLO
// and is read as a bogus frame length, and stale DATA frames on recycled
// stream ids desync the control stream within seconds.
//
// Both ends write the barrier first, then discard inbound frames until they
// see the peer's barrier - everything before it is guaranteed stale because
// the channel preserves per-direction ordering.
var barrierFrame = []byte{
	0xFF, 'O', 'L', 'C', 'R', 'T', 'C', '-',
	'B', 'A', 'R', 'R', 'I', 'E', 'R', '-',
	'v', '1', '-', 's', 'e', 's', 's', 'i',
	'o', 'n', '-', 'r', 'e', 's', 'y', 'n',
}

// WriteBarrier emits the session barrier frame. It must be the first write
// on a fresh conn, before smux is created over it.
func WriteBarrier(conn *muxconn.Conn) error {
	_, err := conn.Write(barrierFrame)
	return err
}

// DrainUntilBarrier discards inbound frames until the peer barrier arrives,
// the frame cap is hit, or the timeout elapses. Reports whether the barrier
// was seen: a peer that never sends one is either old or never restarted,
// and the caller proceeds with pre-barrier behaviour.
func DrainUntilBarrier(conn *muxconn.Conn, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for range barrierMaxFrames {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		frame, ok := conn.NextFrame(remaining)
		if !ok {
			return false
		}
		if bytes.Equal(frame, barrierFrame) {
			return true
		}
	}
	return false
}

package tunnelcore

import (
	"context"
	"testing"
	"time"

	"github.com/openlibrecommunity/olcrtc/internal/crypto"
	"github.com/openlibrecommunity/olcrtc/internal/muxconn"
	"github.com/openlibrecommunity/olcrtc/internal/transport"
)

type barrierStubLink struct{}

func (barrierStubLink) Connect(context.Context) error      { return nil }
func (barrierStubLink) Close() error                       { return nil }
func (barrierStubLink) Send([]byte) error                  { return nil }
func (barrierStubLink) CanSend() bool                      { return true }
func (barrierStubLink) SetReconnectCallback(func())        {}
func (barrierStubLink) SetShouldReconnect(func() bool)     {}
func (barrierStubLink) SetEndedCallback(func(string))      {}
func (barrierStubLink) WatchConnection(context.Context)    {}
func (barrierStubLink) Reconnect(string)                   {}
func (barrierStubLink) Features() transport.Features       { return transport.Features{} }
func (barrierStubLink) SupportsPeerRouting() bool          { return false }

func barrierKeys(t *testing.T) (*crypto.KeySet, *crypto.KeySet) {
	t.Helper()
	seed := []byte("01234567890123456789012345678901")
	client, err := crypto.NewKeySet(seed, crypto.Client)
	if err != nil {
		t.Fatalf("NewKeySet(client): %v", err)
	}
	server, err := crypto.NewKeySet(seed, crypto.Server)
	if err != nil {
		t.Fatalf("NewKeySet(server): %v", err)
	}
	return client, server
}

func pushCiphertext(t *testing.T, conn *muxconn.Conn, keys *crypto.KeySet, plain []byte) {
	t.Helper()
	frame, err := keys.Seal(plain, []byte("olcrtc/muxconn/v2/data"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	conn.Push(frame)
}

func TestDrainUntilBarrierSkipsStaleFrames(t *testing.T) {
	clientKeys, serverKeys := barrierKeys(t)
	conn := muxconn.New(barrierStubLink{}, serverKeys)
	pushCiphertext(t, conn, clientKeys, []byte(`{"addr":"10.0.0.1"}`))
	pushCiphertext(t, conn, clientKeys, []byte("stale-smux-junk"))
	barrier, err := clientKeys.Seal(barrierFrame, []byte("olcrtc/muxconn/v2/data"))
	if err != nil {
		t.Fatalf("Seal barrier: %v", err)
	}
	conn.Push(barrier)
	pushCiphertext(t, conn, clientKeys, []byte("fresh-syn"))
	if !DrainUntilBarrier(conn, time.Second) {
		t.Fatal("barrier not seen")
	}
	frame, ok := conn.NextFrame(time.Second)
	if !ok || string(frame) != "fresh-syn" {
		t.Fatalf("post-barrier frame = %q ok=%v", frame, ok)
	}
}

func TestDrainUntilBarrierTimeoutWithoutBarrier(t *testing.T) {
	clientKeys, serverKeys := barrierKeys(t)
	conn := muxconn.New(barrierStubLink{}, serverKeys)
	pushCiphertext(t, conn, clientKeys, []byte("old-peer-frame"))
	start := time.Now()
	if DrainUntilBarrier(conn, 50*time.Millisecond) {
		t.Fatal("unexpected barrier")
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("drain returned before timeout")
	}
}

func TestWriteBarrierIsDecryptableByPeer(t *testing.T) {
	clientKeys, serverKeys := barrierKeys(t)
	var captured []byte
	ln := captureLink{sink: func(b []byte) { captured = append([]byte(nil), b...) }}
	writer := muxconn.New(ln, serverKeys)
	if err := WriteBarrier(writer); err != nil {
		t.Fatalf("WriteBarrier: %v", err)
	}
	reader := muxconn.New(barrierStubLink{}, clientKeys)
	reader.Push(captured)
	if !DrainUntilBarrier(reader, time.Second) {
		t.Fatal("peer barrier not recognised")
	}
}

type captureLink struct {
	barrierStubLink
	sink func([]byte)
}

func (l captureLink) Send(b []byte) error { l.sink(b); return nil }

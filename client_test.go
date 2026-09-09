package rustpbxgo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func quietLogger() *logrus.Logger {
	l := logrus.New()
	l.Out = io.Discard
	l.Level = logrus.ErrorLevel
	return l
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func newTestServer(t *testing.T, handler func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, srvURL string) (*Client, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient(wsURL(srvURL), WithContext(ctx), WithLogger(quietLogger()))
	return c, cancel
}

func mustConnect(t *testing.T, c *Client) {
	t.Helper()
	if err := c.Connect("websocket"); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
}

// TestEventsProcessedBeforeOnClose: the server sends events followed by the
// hangup and then drops the connection. OnClose must fire only AFTER every
// received event (including hangup) has been fully processed.
func TestEventsProcessedBeforeOnClose(t *testing.T) {
	const total = 10
	srv := newTestServer(t, func(conn *websocket.Conn) {
		for i := 0; i < total-1; i++ {
			_ = conn.WriteMessage(websocket.TextMessage,
				[]byte(fmt.Sprintf(`{"event":"ping","timestamp":%d}`, i)))
		}
		_ = conn.WriteMessage(websocket.TextMessage,
			[]byte(`{"event":"hangup","trackId":"t","timestamp":99}`))
		// Abrupt TCP close (no websocket close handshake): forces a read
		// error on the client, exactly like the server dying after a BYE.
	})

	var processed, hangup int32
	onCloseCh := make(chan string, 1)
	c, cancel := newTestClient(t, srv.URL)
	defer cancel()
	c.OnPing = func(PingEvent) { atomic.AddInt32(&processed, 1) }
	c.OnHangup = func(HangupEvent) {
		atomic.AddInt32(&hangup, 1)
		atomic.AddInt32(&processed, 1)
	}
	c.OnClose = func(reason string) { onCloseCh <- reason }
	mustConnect(t, c)

	select {
	case reason := <-onCloseCh:
		if reason == "" {
			t.Fatal("empty close reason")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnClose not fired within 5s")
	}
	// Happens-before with OnClose is provided by eventWG.Wait(); the race
	// detector validates this ordering too.
	if got := atomic.LoadInt32(&processed); got != total {
		t.Fatalf("processed=%d, want %d (events lost or OnClose fired early)", got, total)
	}
	if atomic.LoadInt32(&hangup) != 1 {
		t.Fatal("hangup event was not processed before close")
	}
}

// TestShutdownSkipsOnCloseAndDoesNotDeadlock: Shutdown must never fire OnClose
// (original behavior) and must terminate promptly even with handlers in
// flight.
func TestShutdownSkipsOnCloseAndDoesNotDeadlock(t *testing.T) {
	const total = 200
	srv := newTestServer(t, func(conn *websocket.Conn) {
		for i := 0; i < total; i++ {
			if err := conn.WriteMessage(websocket.TextMessage,
				[]byte(fmt.Sprintf(`{"event":"ping","timestamp":%d}`, i))); err != nil {
				return
			}
		}
		// Hold the connection open until the client disconnects.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	var processed, onCloseCalled int32
	c, cancel := newTestClient(t, srv.URL)
	defer cancel()
	c.OnPing = func(PingEvent) {
		atomic.AddInt32(&processed, 1)
		time.Sleep(10 * time.Millisecond) // keep handlers in flight
	}
	c.OnClose = func(string) { atomic.AddInt32(&onCloseCalled, 1) }
	mustConnect(t, c)

	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt32(&processed) < total && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&processed); got != total {
		t.Fatalf("processed=%d before shutdown, want %d", got, total)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Shutdown()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return within 5s (deadlock)")
	}

	time.Sleep(300 * time.Millisecond)
	if n := atomic.LoadInt32(&onCloseCalled); n != 0 {
		t.Fatalf("OnClose fired %d times on Shutdown, want 0 (original behavior)", n)
	}
}

// TestNoDropUnderBurstWithBlockedWrites: the server never reads, so the
// client's command writes block; meanwhile it floods events. The event path
// must never drop messages (backpressure instead of the old default: drop).
func TestNoDropUnderBurstWithBlockedWrites(t *testing.T) {
	const total = 1000
	srv := newTestServer(t, func(conn *websocket.Conn) {
		padding := strings.Repeat("x", 2048)
		for i := 0; i < total; i++ {
			msg := fmt.Sprintf(`{"event":"ping","timestamp":%d,"payload":"%s"}`, i, padding)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
		}
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	var processed int32
	c, cancel := newTestClient(t, srv.URL)
	defer cancel()
	c.OnPing = func(PingEvent) { atomic.AddInt32(&processed, 1) }
	mustConnect(t, c)

	// Jam the command path: big commands the server never reads. With the
	// old single shared loop this stalls event processing (eventChan fills,
	// the read loop drops on `default:`).
	go func() {
		big := strings.Repeat("y", 1024)
		for i := 0; i < 300; i++ {
			if err := c.sendCommand(map[string]any{"command": "noop", "pad": big}); err != nil {
				return
			}
		}
	}()

	deadline := time.Now().Add(15 * time.Second)
	for atomic.LoadInt32(&processed) < total && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&processed); got != total {
		t.Fatalf("processed=%d, want %d — events were dropped under burst", got, total)
	}
}

// TestShutdownWhileFloodingDoesNotDeadlock: regression for the drain path —
// the read loop must close eventChan on every exit path so the event loop's
// drain terminates, even when Shutdown lands while the channel is full.
func TestShutdownWhileFloodingDoesNotDeadlock(t *testing.T) {
	srv := newTestServer(t, func(conn *websocket.Conn) {
		for i := 0; ; i++ {
			msg := []byte(fmt.Sprintf(`{"event":"ping","timestamp":%d}`, i))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	})

	var processed, onCloseCalled int32
	c, cancel := newTestClient(t, srv.URL)
	defer cancel()
	c.OnPing = func(PingEvent) { atomic.AddInt32(&processed, 1) }
	c.OnClose = func(string) { atomic.AddInt32(&onCloseCalled, 1) }
	mustConnect(t, c)

	time.Sleep(200 * time.Millisecond) // let the flood fill the pipeline

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_ = c.Shutdown()
		// Once teardown completes, cancel() has run: sendCommand must fail
		// instead of blocking. If it keeps succeeding, the event loop never
		// finished draining — a deadlock.
		for i := 0; i < 500; i++ {
			if err := c.sendCommand(Command{Command: "noop"}); err != nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Error("sendCommand still succeeding after 2.5s — event loop teardown stuck (deadlock)")
	}()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: shutdown did not complete within 10s")
	}

	time.Sleep(200 * time.Millisecond)
	if n := atomic.LoadInt32(&onCloseCalled); n != 0 {
		t.Fatalf("OnClose fired %d times on Shutdown, want 0", n)
	}
}

// TestInviteTapConsumeAndFallback: Invite consumes the first answer via the
// internal tap WITHOUT touching user callbacks; a hangup sent after Invite
// returned falls through to the user's OnHangup. Run under -race to validate
// the tap's atomicity.
func TestInviteTapConsumeAndFallback(t *testing.T) {
	srv := newTestServer(t, func(conn *websocket.Conn) {
		for i := 0; i < 50; i++ {
			_ = conn.WriteMessage(websocket.TextMessage,
				[]byte(fmt.Sprintf(`{"event":"ping","timestamp":%d}`, i)))
		}
		_ = conn.WriteMessage(websocket.TextMessage,
			[]byte(`{"event":"answer","trackId":"t","timestamp":51,"sdp":"v=0-answer"}`))
		// Send the hangup only after the client signals (by sending a
		// command) that Invite returned and the tap is cleared, so the
		// event must fall through to the user callback deterministically.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			_ = conn.WriteMessage(websocket.TextMessage,
				[]byte(`{"event":"hangup","trackId":"t","timestamp":52,"reason":"done"}`))
		}
	})

	var userAnswerCalled, userHangupCalled int32
	c, cancel := newTestClient(t, srv.URL)
	defer cancel()
	c.onAnswer = func(AnswerEvent) { atomic.AddInt32(&userAnswerCalled, 1) }
	c.OnHangup = func(HangupEvent) { atomic.AddInt32(&userHangupCalled, 1) }
	mustConnect(t, c)

	ctx, cancelInvite := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelInvite()
	answer, err := c.Invite(ctx, CallOption{})
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if answer.Sdp != "v=0-answer" {
		t.Fatalf("answer sdp=%q", answer.Sdp)
	}
	if n := atomic.LoadInt32(&userAnswerCalled); n != 0 {
		t.Fatal("user OnAnswer must not be called for the answer consumed by Invite")
	}

	// Nudge the server to emit the hangup now that the tap is cleared.
	_ = c.sendCommand(map[string]any{"command": "proceed"})

	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt32(&userHangupCalled) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if atomic.LoadInt32(&userHangupCalled) == 0 {
		t.Fatal("hangup after answer must fall through to user OnHangup")
	}
}

// TestConcurrentInviteAndShutdown: hammer the tap registration, event flood
// and shutdown together; -race validates there is no data race.
func TestConcurrentInviteAndShutdown(t *testing.T) {
	srv := newTestServer(t, func(conn *websocket.Conn) {
		for i := 0; ; i++ {
			msg := []byte(fmt.Sprintf(`{"event":"ping","timestamp":%d}`, i))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	})

	c, cancel := newTestClient(t, srv.URL)
	defer cancel()
	mustConnect(t, c)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			_, _ = c.Invite(ctx, CallOption{}) // will mostly time out; fine
		}()
	}
	time.Sleep(300 * time.Millisecond)
	_ = c.Shutdown()
	wg.Wait()
}

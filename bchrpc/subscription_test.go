package bchrpc

import (
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newDispatcherTestServer returns a server running only its event
// dispatcher.
func newDispatcherTestServer(t *testing.T) *GrpcServer {
	t.Helper()
	s := &GrpcServer{
		subscribe:   make(chan *rpcEventSubscription),
		unsubscribe: make(chan *rpcEventSubscription),
		events:      make(chan interface{}),
		quit:        make(chan struct{}),
	}
	s.wg.Add(1)
	go s.runEventDispatcher()
	t.Cleanup(func() {
		close(s.quit)
		s.wg.Wait()
	})
	return s
}

// TestSubscriptionTeardown ensures unsubscribing stops the subscription's
// goroutine and removes it from the dispatcher, without waiting for an event.
func TestSubscriptionTeardown(t *testing.T) {
	s := newDispatcherTestServer(t)

	const n = 50
	for range n {
		sub := s.subscribeEvents(maxSubscriptionQueue)
		sub.Unsubscribe()

		// The queue handler closes done as it returns.
		<-sub.done
	}

	// The dispatcher handles one message at a time, so once it has taken
	// this unsubscribe of a subscription it does not hold, which changes
	// nothing, it is done with the last real one.
	s.unsubscribe <- &rpcEventSubscription{}
	if got := s.numSubscriptions(); got != 0 {
		t.Errorf("%d subscriptions left in the dispatcher", got)
	}
}

// TestSubscriptionQueueBounded ensures a subscriber that stops reading is
// dropped once its queue is full, instead of buffering every event.
func TestSubscriptionQueueBounded(t *testing.T) {
	s := newDispatcherTestServer(t)
	slow := s.subscribeEvents(maxSubscriptionQueue)
	defer slow.Unsubscribe()
	fast := s.subscribeEvents(maxSubscriptionQueue)
	defer fast.Unsubscribe()

	received := make(chan int)
	go func() {
		n := 0
		for range fast.Events() {
			n++
			if n == maxSubscriptionQueue+10 {
				break
			}
		}
		received <- n
	}()
	for i := 0; i < maxSubscriptionQueue+10; i++ {
		s.dispatchEvent(i)
	}
	if n := <-received; n != maxSubscriptionQueue+10 {
		t.Fatalf("reading subscriber got %d events", n)
	}

	select {
	case _, ok := <-slow.Events():
		if ok {
			t.Fatal("subscriber that stopped reading still has queued events")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("events channel not closed")
	}
	if got := status.Code(slow.closedErr()); got != codes.ResourceExhausted {
		t.Errorf("closed with %v, want ResourceExhausted", got)
	}
}

// TestQueueHandlerUnbounded ensures a queue with no limit keeps every event
// for a reader that is slow to start.
func TestQueueHandlerUnbounded(t *testing.T) {
	in := make(chan interface{})
	out := make(chan interface{})
	quit := make(chan struct{})
	done := make(chan bool)
	go func() { done <- queueHandler(in, out, quit, nil, 0) }()

	const n = maxSubscriptionQueue + 10
	for i := 0; i < n; i++ {
		in <- i
	}
	for i := 0; i < n; i++ {
		if got := <-out; got != i {
			t.Fatalf("event %d: got %v", i, got)
		}
	}
	close(in)
	if !<-done {
		t.Error("queue reported an overflow")
	}
}

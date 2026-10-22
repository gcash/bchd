package bchrpc

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg"
	"google.golang.org/grpc"
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

// txStream is a SubscribeTransactionStream stream whose client has sent reqs
// and then sends nothing until the stream ends.
type txStream struct {
	grpc.ServerStream
	ctx  context.Context
	reqs []*pb.SubscribeTransactionsRequest
}

func (s *txStream) Context() context.Context { return s.ctx }

func (s *txStream) Send(*pb.TransactionNotification) error { return nil }

func (s *txStream) Recv() (*pb.SubscribeTransactionsRequest, error) {
	if len(s.reqs) > 0 {
		req := s.reqs[0]
		s.reqs = s.reqs[1:]
		return req, nil
	}
	<-s.ctx.Done()
	return nil, s.ctx.Err()
}

// streamReaders returns the number of goroutines reading the requests of a
// SubscribeTransactionStream call.
func streamReaders() int {
	buf := make([]byte, 1<<16)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	return strings.Count(string(buf),
		"(*GrpcServer).SubscribeTransactionStream.func1(")
}

// TestSubscribeTransactionStreamReaderStops ensures the goroutine reading the
// requests of a SubscribeTransactionStream call stops once the call ends,
// here on a bad filter while it holds the client's next request, instead of
// waiting forever to hand that request over.
func TestSubscribeTransactionStreamReaderStops(t *testing.T) {
	s := newDispatcherTestServer(t)
	s.chainParams = &chaincfg.RegressionNetParams
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &txStream{ctx: ctx, reqs: []*pb.SubscribeTransactionsRequest{
		{Subscribe: &pb.TransactionFilter{Addresses: []string{"bad"}}},
		{IncludeMempool: true},
	}}
	if err := s.SubscribeTransactionStream(stream); err == nil {
		t.Fatal("a filter with a bad address was accepted")
	}

	// gRPC ends the stream's context as the handler returns.
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for streamReaders() > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the request reader still runs after the call ended")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

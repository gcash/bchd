package bchrpc

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/mempool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newDispatcherTestServer returns a server running only its event
// dispatcher.
func newDispatcherTestServer(t *testing.T) *GrpcServer {
	t.Helper()
	s := &GrpcServer{
		subscribe:        make(chan *rpcEventSubscription),
		unsubscribe:      make(chan *rpcEventSubscription),
		events:           make(chan interface{}),
		quit:             make(chan struct{}),
		maxSubscriptions: 10,
		maxFilterEntries: maxFilterSize,
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

// blockStream is a SubscribeBlocks stream that takes every notification.
type blockStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *blockStream) Context() context.Context { return s.ctx }

func (s *blockStream) Send(*pb.BlockNotification) error { return nil }

// waitSubscriptions waits until the dispatcher holds n subscriptions.
func waitSubscriptions(t *testing.T, s *GrpcServer, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for s.numSubscriptions() != n {
		if time.Now().After(deadline) {
			t.Fatalf("%d subscriptions in the dispatcher, want %d",
				s.numSubscriptions(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestClientSubscriptionLimit ensures clients hold at most maxSubscriptions
// subscriptions at once, whatever the calls, that a call over the limit is
// refused at once with ResourceExhausted, and that a subscription that ends
// frees its place.  The server's own subscriptions do not count.
func TestClientSubscriptionLimit(t *testing.T) {
	s := newDispatcherTestServer(t)
	s.chainParams = &chaincfg.RegressionNetParams
	s.txMemPool = mempool.New(&mempool.Config{})
	s.maxSubscriptions = 2

	own := s.subscribeEvents(0)
	defer own.Unsubscribe()

	type call struct {
		cancel context.CancelFunc
		done   chan error
	}
	subscribe := func() call {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		c := call{cancel: cancel, done: make(chan error, 1)}
		go func() {
			c.done <- s.SubscribeBlocks(&pb.SubscribeBlocksRequest{},
				&blockStream{ctx: ctx})
		}()
		return c
	}
	first, second := subscribe(), subscribe()
	defer first.cancel()
	defer second.cancel()
	waitSubscriptions(t, s, 3)

	calls := []struct {
		name string
		call func(context.Context) error
	}{
		{"SubscribeTransactions", func(ctx context.Context) error {
			return s.SubscribeTransactions(&pb.SubscribeTransactionsRequest{},
				&txStream{ctx: ctx})
		}},
		{"SubscribeTransactionStream", func(ctx context.Context) error {
			return s.SubscribeTransactionStream(&txStream{ctx: ctx})
		}},
		{"SubscribeBlocks", func(ctx context.Context) error {
			return s.SubscribeBlocks(&pb.SubscribeBlocksRequest{},
				&blockStream{ctx: ctx})
		}},
		{"SubscribeDoubleSpendProofs", func(ctx context.Context) error {
			return s.SubscribeDoubleSpendProofs(
				&pb.SubscribeDoubleSpendProofsRequest{},
				&dsProofStream{ctx: ctx})
		}},
	}
	for _, c := range calls {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := c.call(ctx)
		cancel()
		if status.Code(err) != codes.ResourceExhausted {
			t.Errorf("%s over the limit: got %v, want ResourceExhausted",
				c.name, err)
		}
	}

	// The refused calls took no place, and one that ends frees its own.
	first.cancel()
	if err := <-first.done; err != nil {
		t.Fatalf("the first subscription ended with %v", err)
	}
	third := subscribe()
	defer third.cancel()
	waitSubscriptions(t, s, 3)
	select {
	case err := <-third.done:
		t.Fatalf("a subscription after one ended was refused: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestSubscribeTransactionStreamFilterLimit ensures a SubscribeTransactionStream
// call ends with ResourceExhausted once its requests together add more than
// maxFilterSize to its filter, as each request may add up to gRPC's message
// limit and the call can send any number of them.
func TestSubscribeTransactionStreamFilterLimit(t *testing.T) {
	s := newDispatcherTestServer(t)
	s.chainParams = &chaincfg.RegressionNetParams
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &txStream{ctx: ctx, reqs: []*pb.SubscribeTransactionsRequest{
		{Subscribe: filterOutpoints(1, maxFilterSize/2)},
		{Subscribe: filterOutpoints(2, maxFilterSize/2)},
		{Subscribe: filterOutpoints(3, 1)},
	}}
	done := make(chan error, 1)
	go func() { done <- s.SubscribeTransactionStream(stream) }()
	select {
	case err := <-done:
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("got %v, want ResourceExhausted", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the call still runs with more than maxFilterSize in its filter")
	}
}

// TestNewGrpcServerMaxSubscriptions ensures the server takes its subscription
// and filter limits from its config.
func TestNewGrpcServerMaxSubscriptions(t *testing.T) {
	saved := serviceMap["pb.bchrpc"]
	defer func() { serviceMap["pb.bchrpc"] = saved }()
	s := NewGrpcServer(&GrpcServerConfig{Server: grpc.NewServer(),
		MaxSubscriptions: 7, MaxFilterEntries: 11})
	if s.maxSubscriptions != 7 {
		t.Errorf("limit %d, want 7", s.maxSubscriptions)
	}
	if s.maxFilterEntries != 11 {
		t.Errorf("filter limit %d, want 11", s.maxFilterEntries)
	}
}

// TestSubscribeTransactionsConfiguredFilterLimit ensures a SubscribeTransactions
// request whose filter is over the server's configured limit is refused with
// ResourceExhausted, naming the limit.
func TestSubscribeTransactionsConfiguredFilterLimit(t *testing.T) {
	s := newDispatcherTestServer(t)
	s.chainParams = &chaincfg.RegressionNetParams
	s.maxFilterEntries = 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- s.SubscribeTransactions(&pb.SubscribeTransactionsRequest{
			Subscribe: filterOutpoints(1, 4)}, &txStream{ctx: ctx})
	}()
	select {
	case err := <-done:
		if status.Code(err) != codes.ResourceExhausted ||
			!strings.Contains(err.Error(), "more than 3 entries") {

			t.Fatalf("got %v, want ResourceExhausted at 3 entries", err)
		}
	case <-time.After(time.Second):
		t.Fatal("a filter over the limit was not refused")
	}
}

// TestSubscribeTransactionStreamConfiguredFilterLimit ensures a
// SubscribeTransactionStream call holds its filter to the server's configured
// limit: requests that fill it are kept, and one more entry ends the call with
// ResourceExhausted, naming the limit.
func TestSubscribeTransactionStreamConfiguredFilterLimit(t *testing.T) {
	for _, over := range []bool{false, true} {
		s := newDispatcherTestServer(t)
		s.chainParams = &chaincfg.RegressionNetParams
		s.maxFilterEntries = 3
		ctx, cancel := context.WithCancel(context.Background())
		reqs := []*pb.SubscribeTransactionsRequest{
			{Subscribe: filterOutpoints(1, 2)},
			{Subscribe: filterOutpoints(2, 1)},
		}
		if over {
			reqs = append(reqs, &pb.SubscribeTransactionsRequest{
				Subscribe: filterOutpoints(3, 1)})
		}
		stream := &txStream{ctx: ctx, reqs: reqs}
		done := make(chan error, 1)
		go func() { done <- s.SubscribeTransactionStream(stream) }()
		select {
		case err := <-done:
			if !over {
				t.Fatalf("a filter at the limit ended the call: %v", err)
			}
			if status.Code(err) != codes.ResourceExhausted ||
				!strings.Contains(err.Error(), "more than 3 entries") {

				t.Fatalf("got %v, want ResourceExhausted at 3 "+
					"entries", err)
			}
		case <-time.After(time.Second):
			if over {
				t.Fatal("the call still runs with more than 3 " +
					"entries in its filter")
			}
		}
		cancel()
	}
}

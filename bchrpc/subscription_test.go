package bchrpc

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/wire"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
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
		sub := s.subscribeEvents(maxSubscriptionQueue, rpcEventsAll)
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
	slow := s.subscribeEvents(maxSubscriptionQueue, rpcEventsAll)
	defer slow.Unsubscribe()
	fast := s.subscribeEvents(maxSubscriptionQueue, rpcEventsAll)
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
// and then sends nothing until the stream ends.  The client's side ends with
// the stream, as io.EOF, so that the call's request reader stops without
// logging an error: it can stop after the call has returned, and so after the
// test has, while another test swaps the logger.
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
	return nil, io.EOF
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

	own := s.subscribeEvents(0, rpcEventsAll)
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

// testPeerAddr is a client's address as grpc-go's ServeHTTP transport gives
// it, the request's remote address.
type testPeerAddr string

func (a testPeerAddr) Network() string { return "tcp" }

func (a testPeerAddr) String() string { return string(a) }

// TestClientSubscriptionShare ensures a client holds at most its share of the
// subscriptions, so one client cannot take every place: a client is an IPv4
// address, also when mapped into IPv6, or the /56 of an IPv6 address.  Other
// clients still subscribe, and a subscription that ends gives its place back.
// Clients at loopback, private and link-local addresses, which are usually a
// proxy in front of the server, have no share of their own, nor do those
// whose address cannot be read, and with an auth token no client has one.
func TestClientSubscriptionShare(t *testing.T) {
	s := newDispatcherTestServer(t)
	s.maxSubscriptions = 100
	s.maxClientShare = 2

	type held struct {
		cancel context.CancelFunc
		done   chan error
	}
	var subs []held
	defer func() {
		for _, sub := range subs {
			sub.cancel()
		}
	}()
	// subscribe makes a SubscribeBlocks call from addr, or from a client
	// without an address when addr is empty, and returns its error once
	// it is refused, or nil once it holds a place.
	subscribe := func(addr string) error {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		if addr != "" {
			ctx = peer.NewContext(ctx, &peer.Peer{Addr: testPeerAddr(addr)})
		}
		// The dispatcher handles one message at a time, so once it has
		// taken this unsubscribe of a subscription it does not hold, it
		// counts every subscription that ended.
		s.unsubscribe <- &rpcEventSubscription{}
		n := s.numSubscriptions()
		done := make(chan error, 1)
		go func() {
			done <- s.SubscribeBlocks(&pb.SubscribeBlocksRequest{},
				&blockStream{ctx: ctx})
		}()
		for deadline := time.Now().Add(5 * time.Second); ; {
			select {
			case err := <-done:
				cancel()
				return err
			default:
			}
			if s.numSubscriptions() > n {
				subs = append(subs, held{cancel, done})
				return nil
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: the call neither subscribed nor ended", addr)
			}
			time.Sleep(time.Millisecond)
		}
	}
	refused := func(addr string) {
		t.Helper()
		err := subscribe(addr)
		if status.Code(err) != codes.ResourceExhausted ||
			!strings.Contains(err.Error(), "from one client, the limit is 2") {

			t.Errorf("%s over its share: got %v, want ResourceExhausted",
				addr, err)
		}
	}
	accepted := func(addrs ...string) {
		t.Helper()
		for _, addr := range addrs {
			if err := subscribe(addr); err != nil {
				t.Errorf("%s: refused: %v", addr, err)
			}
		}
	}

	accepted("203.0.113.5:1000", "203.0.113.5:1001")
	refused("203.0.113.5:1002")
	refused("[::ffff:203.0.113.5]:1003")
	accepted("203.0.113.6:1000", "203.0.113.6:1001")
	accepted("198.51.100.7:1000", "198.51.100.7:1001")
	accepted("[2001:db8:1:2::1]:1000", "[2001:db8:1:3:ffff::1]:1001")
	refused("[2001:db8:1:ff::abcd]:1002")
	accepted("[2001:db8:1:100::1]:1000")
	for _, addr := range []string{"127.0.0.1:1000", "[::1]:1000",
		"10.1.2.3:1000", "172.16.0.1:1000", "192.168.0.1:1000",
		"[fd00::1]:1000", "169.254.1.1:1000", "[fe80::1]:1000",
		"not an address", ""} {

		accepted(addr, addr, addr)
	}

	// A subscription that ends gives its place in the share back.
	subs[0].cancel()
	if err := <-subs[0].done; err != nil {
		t.Fatalf("the first subscription ended with %v", err)
	}
	subs = subs[1:]
	accepted("203.0.113.5:1004")
	refused("203.0.113.5:1005")

	// A call refused by the overall limit takes no place in its client's
	// share.
	s.maxSubscriptions = int(s.clientSubscriptions.Load())
	if err := subscribe("203.0.113.7:1000"); status.Code(err) != codes.ResourceExhausted ||
		!strings.Contains(err.Error(), "too many subscriptions, the limit") {

		t.Errorf("over the overall limit: got %v, want ResourceExhausted", err)
	}
	s.maxSubscriptions = 100
	accepted("203.0.113.7:1001", "203.0.113.7:1002")

	// With an auth token, clients have no share of their own.
	s.maxClientShare = 0
	accepted("203.0.113.9:1000", "203.0.113.9:1001", "203.0.113.9:1002")

	// Once every subscription has ended, no client is counted.
	for _, sub := range subs {
		sub.cancel()
		if err := <-sub.done; err != nil {
			t.Fatalf("a subscription ended with %v", err)
		}
	}
	subs = nil
	s.clientSharesMu.Lock()
	left := len(s.clientShares)
	s.clientSharesMu.Unlock()
	if left != 0 {
		t.Errorf("%d clients still counted", left)
	}
	if n := s.clientSubscriptions.Load(); n != 0 {
		t.Errorf("%d subscriptions still counted", n)
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

	// The call's request reader ends before the test does.
	cancel()
	waitStreamReaders(t)
}

// reqTxStream is a SubscribeTransactionStream stream whose client sends the
// requests handed to reqs, and which passes on the notifications it is sent.
// The client's side ends with the stream, as txStream's does.
type reqTxStream struct {
	grpc.ServerStream
	ctx  context.Context
	reqs chan *pb.SubscribeTransactionsRequest
	sent chan *pb.TransactionNotification
}

func (s *reqTxStream) Context() context.Context { return s.ctx }

func (s *reqTxStream) Recv() (*pb.SubscribeTransactionsRequest, error) {
	select {
	case req := <-s.reqs:
		return req, nil
	case <-s.ctx.Done():
		return nil, io.EOF
	}
}

func (s *reqTxStream) Send(n *pb.TransactionNotification) error {
	s.sent <- n
	return nil
}

// TestSubscribeTransactionStreamMatchedOutputsNotCounted ensures the outputs a
// SubscribeTransactionStream call's filter matched do not count toward the
// filter's limit, so a later request naming an entry does not end the call
// however many outputs the filter matched.
func TestSubscribeTransactionStreamMatchedOutputsNotCounted(t *testing.T) {
	s := newDispatcherTestServer(t)
	s.chainParams = &chaincfg.RegressionNetParams
	s.maxFilterEntries = 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &reqTxStream{ctx: ctx,
		reqs: make(chan *pb.SubscribeTransactionsRequest),
		sent: make(chan *pb.TransactionNotification, 10)}
	done := make(chan error, 1)
	go func() { done <- s.SubscribeTransactionStream(stream) }()

	// The call takes a request only once it has handled the one before,
	// so once three are taken, the first has been handled.
	data := []byte{1, 2, 3}
	watch := &pb.SubscribeTransactionsRequest{
		Subscribe:      &pb.TransactionFilter{DataElements: [][]byte{data}},
		IncludeMempool: true,
		SerializeTx:    true,
	}
	send := func(what string) {
		t.Helper()
		for i := 0; i < 3; i++ {
			select {
			case stream.reqs <- watch:
			case err := <-done:
				t.Fatalf("%s: the call ended: %v", what, err)
			case <-time.After(5 * time.Second):
				t.Fatalf("%s: the call took no request", what)
			}
		}
	}
	send("the filter")
	for i := 0; i < 5; i++ {
		s.dispatchEvent(txEvent(dataTx(wire.OutPoint{Index: uint32(i)}, data)))
	}
	for i := 0; i < 5; i++ {
		select {
		case <-stream.sent:
		case <-time.After(5 * time.Second):
			t.Fatalf("%d of 5 matching transactions notified", i)
		}
	}
	send("the entry again")

	// The call and its request reader end before the test does.
	cancel()
	if err := <-done; err != nil {
		t.Errorf("the call ended with %v", err)
	}
	waitStreamReaders(t)
}

// waitStreamReaders waits until no goroutine reads the requests of a
// SubscribeTransactionStream call.
func waitStreamReaders(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for streamReaders() > 0 {
		if time.Now().After(deadline) {
			t.Fatal("a request reader still runs after its call ended")
		}
		time.Sleep(10 * time.Millisecond)
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

	// Each client may hold a quarter of the subscriptions, rounded up,
	// unless clients need the auth token.
	for _, test := range []struct {
		max, share int
		auth       bool
	}{{7, 2, false}, {100, 25, false}, {1, 1, false}, {8, 2, false},
		{100, 0, true}} {

		s := NewGrpcServer(&GrpcServerConfig{Server: grpc.NewServer(),
			MaxSubscriptions: test.max, AuthRequired: test.auth})
		if s.maxClientShare != test.share {
			t.Errorf("limit %d, auth %v: share %d, want %d", test.max,
				test.auth, s.maxClientShare, test.share)
		}
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

		// The call, when it still runs, and its request reader end
		// before the next one starts.
		cancel()
		if !over {
			if err := <-done; err != nil {
				t.Errorf("the call ended with %v", err)
			}
		}
		waitStreamReaders(t)
	}
}

// TestSubscriptionEventKinds ensures the dispatcher hands a subscription only
// the kinds of event it takes, so that others neither wait in its queue nor
// end it however many come, that events of a kind it takes still end it once
// more than its queue holds wait, and that it takes the kinds set last.
func TestSubscriptionEventKinds(t *testing.T) {
	s := newDispatcherTestServer(t)
	blocks := s.subscribeEvents(maxSubscriptionQueue,
		rpcEventsBlockConnected|rpcEventsBlockDisconnected)
	defer blocks.Unsubscribe()
	connectedOnly := s.subscribeEvents(maxSubscriptionQueue,
		rpcEventsBlockConnected)
	defer connectedOnly.Unsubscribe()
	disconnectedOnly := s.subscribeEvents(maxSubscriptionQueue,
		rpcEventsBlockDisconnected)
	defer disconnectedOnly.Unsubscribe()
	later := s.subscribeEvents(maxSubscriptionQueue, 0)
	defer later.Unsubscribe()

	for i := 0; i < maxSubscriptionQueue+10; i++ {
		s.dispatchEvent(&rpcEventTxAccepted{})
		s.dispatchEvent(&rpcEventDSProof{})
		s.dispatchEvent(i)
	}
	recv := func(what string, sub *rpcEventSubscription) interface{} {
		t.Helper()
		select {
		case event, ok := <-sub.Events():
			if !ok {
				t.Fatalf("%s: the subscription ended: %v", what,
					sub.closedErr())
			}
			return event
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: no event", what)
		}
		return nil
	}
	connected, disconnected := &rpcEventBlockConnected{}, &rpcEventBlockDisconnected{}
	connected2, disconnected2 := &rpcEventBlockConnected{}, &rpcEventBlockDisconnected{}
	s.dispatchEvent(connected)
	s.dispatchEvent(disconnected)
	s.dispatchEvent(connected2)
	s.dispatchEvent(disconnected2)
	for _, want := range []interface{}{connected, disconnected, connected2} {
		if event := recv("blocks", blocks); event != want {
			t.Errorf("blocks: got %T, want %T", event, want)
		}
	}
	for _, want := range []interface{}{connected, connected2} {
		if event := recv("connected", connectedOnly); event != want {
			t.Errorf("connected: got %T, want %T", event, want)
		}
	}
	for _, want := range []interface{}{disconnected, disconnected2} {
		if event := recv("disconnected", disconnectedOnly); event != want {
			t.Errorf("disconnected: got %T, want %T", event, want)
		}
	}
	later.setKinds(rpcEventsTxAccepted)
	accepted := &rpcEventTxAccepted{}
	s.dispatchEvent(&rpcEventDSProof{})
	s.dispatchEvent(0)
	s.dispatchEvent(&rpcEventBlockConnected{})
	s.dispatchEvent(&rpcEventBlockDisconnected{})
	s.dispatchEvent(accepted)
	if event := recv("later", later); event != accepted {
		t.Errorf("later: got %T first, want the transaction set later", event)
	}

	// More events of a kind it takes than its queue holds end it.
	for i := 0; i < maxSubscriptionQueue+10; i++ {
		s.dispatchEvent(&rpcEventBlockConnected{})
	}
	select {
	case _, ok := <-blocks.Events():
		if ok {
			t.Fatal("blocks: an event left after its queue overflowed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocks: not ended by more blocks than its queue holds")
	}
	if got := status.Code(blocks.closedErr()); got != codes.ResourceExhausted {
		t.Errorf("blocks: ended with %v, want ResourceExhausted", got)
	}
}

// TestTxEventKinds ensures a transaction subscription takes accepted
// transactions only with the memory pool included, and connected blocks only
// with blocks included.
func TestTxEventKinds(t *testing.T) {
	for _, test := range []struct {
		mempool, blocks bool
		want            rpcEventKinds
	}{{false, false, 0}, {true, false, rpcEventsTxAccepted},
		{false, true, rpcEventsBlockConnected},
		{true, true, rpcEventsTxAccepted | rpcEventsBlockConnected}} {

		if got := txEventKinds(test.mempool, test.blocks); got != test.want {
			t.Errorf("mempool %v, blocks %v: kinds %b, want %b",
				test.mempool, test.blocks, got, test.want)
		}
	}
}

// heldSender holds each notification a stream sends until release is closed,
// telling sending as it starts.
type heldSender struct {
	sending chan struct{}
	release chan struct{}
}

func newHeldSender() *heldSender {
	return &heldSender{sending: make(chan struct{}, 100),
		release: make(chan struct{})}
}

func (h *heldSender) send() error {
	h.sending <- struct{}{}
	<-h.release
	return nil
}

// heldTxStream is a transaction subscription's stream whose client sends the
// requests handed to reqs and whose notifications are held.  The client's side
// ends with the stream, as txStream's does.
type heldTxStream struct {
	grpc.ServerStream
	*heldSender
	ctx  context.Context
	reqs chan *pb.SubscribeTransactionsRequest
}

func (s *heldTxStream) Context() context.Context { return s.ctx }

func (s *heldTxStream) Recv() (*pb.SubscribeTransactionsRequest, error) {
	select {
	case req := <-s.reqs:
		return req, nil
	case <-s.ctx.Done():
		return nil, io.EOF
	}
}

func (s *heldTxStream) Send(*pb.TransactionNotification) error { return s.send() }

// heldBlockStream is a SubscribeBlocks stream whose notifications are held.
type heldBlockStream struct {
	grpc.ServerStream
	*heldSender
	ctx context.Context
}

func (s *heldBlockStream) Context() context.Context { return s.ctx }

func (s *heldBlockStream) Send(*pb.BlockNotification) error { return s.send() }

// heldDSProofStream is a SubscribeDoubleSpendProofs stream whose notifications
// are held.
type heldDSProofStream struct {
	grpc.ServerStream
	*heldSender
	ctx context.Context
}

func (s *heldDSProofStream) Context() context.Context { return s.ctx }

func (s *heldDSProofStream) Send(*pb.DoubleSpendProofNotification) error {
	return s.send()
}

// checkOtherEventsKept ensures that the subscription made by call, the only
// one of s, once held in the send of its notification of an event trigger
// makes, is not ended by more events of each of the kinds others makes than
// its queue holds, which it does not take: once released it sends trigger's
// notification again.  It returns a function that waits for the call to end
// once its context ends.
func checkOtherEventsKept(t *testing.T, name string, s *GrpcServer,
	held *heldSender, call func() error, trigger func() interface{},
	others ...func() interface{}) func() {

	t.Helper()
	done := make(chan error, 1)
	go func() { done <- call() }()
	waitSubscriptions(t, s, 1)
	ended := false
	sent := func(what string) bool {
		t.Helper()
		s.dispatchEvent(trigger())
		select {
		case <-held.sending:
			return true
		case err := <-done:
			ended = true
			t.Errorf("%s: ended before its notification %s: %v", name,
				what, err)
		case <-time.After(5 * time.Second):
			t.Errorf("%s: no notification %s", name, what)
		}
		return false
	}
	wait := func() {
		if ended {
			return
		}
		if err := <-done; err != nil {
			t.Errorf("%s: ended with %v", name, err)
		}
	}
	if !sent("first") {
		return wait
	}
	for _, other := range others {
		for i := 0; i < maxSubscriptionQueue+10; i++ {
			s.dispatchEvent(other())
		}
	}
	close(held.release)
	sent("after the other events")
	return wait
}

// TestSubscriptionsTakeTheirEventKinds ensures each subscription call takes
// only the kinds of event it sends notifications of, so that a subscriber
// busy with a notification is not ended by more events of other kinds than
// its queue holds: a block subscriber by memory pool transactions or proofs,
// a transaction subscriber without blocks by blocks or proofs, and a proof
// subscriber by transactions or blocks.
func TestSubscriptionsTakeTheirEventKinds(t *testing.T) {
	data := []byte{1, 2, 3}
	watch := &pb.SubscribeTransactionsRequest{
		Subscribe:      &pb.TransactionFilter{DataElements: [][]byte{data}},
		IncludeMempool: true,
		SerializeTx:    true,
	}
	n := uint32(0)
	accepted := func() interface{} {
		n++
		return txEvent(dataTx(wire.OutPoint{Index: n}, data))
	}
	connected := func() interface{} { return &rpcEventBlockConnected{} }
	disconnected := func() interface{} { return &rpcEventBlockDisconnected{} }
	proof := func() interface{} {
		return &rpcEventDSProof{&mempool.DSProofDesc{Proof: &wire.MsgDSProof{}}}
	}

	newServer := func() *GrpcServer {
		s := newDispatcherTestServer(t)
		s.chainParams = &chaincfg.RegressionNetParams
		s.txMemPool = mempool.New(&mempool.Config{})
		return s
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls []func()
	s, held := newServer(), newHeldSender()
	calls = append(calls, checkOtherEventsKept(t, "SubscribeTransactions", s, held, func() error {
		return s.SubscribeTransactions(watch, &heldTxStream{ctx: ctx,
			heldSender: held})
	}, accepted, connected, disconnected, proof))

	// The stream takes events once it has handled its first request,
	// which it has once it takes the third.
	s, held = newServer(), newHeldSender()
	reqs := make(chan *pb.SubscribeTransactionsRequest)
	stream := &heldTxStream{ctx: ctx, heldSender: held, reqs: reqs}
	streamDone := make(chan error, 1)
	go func() { streamDone <- s.SubscribeTransactionStream(stream) }()
	for i := 0; i < 3; i++ {
		select {
		case reqs <- watch:
		case err := <-streamDone:
			t.Fatalf("SubscribeTransactionStream ended: %v", err)
		}
	}
	calls = append(calls, checkOtherEventsKept(t, "SubscribeTransactionStream", s, held, func() error {
		return <-streamDone
	}, accepted, connected, disconnected, proof))

	s, held = newServer(), newHeldSender()
	calls = append(calls, checkOtherEventsKept(t, "SubscribeDoubleSpendProofs", s, held, func() error {
		return s.SubscribeDoubleSpendProofs(
			&pb.SubscribeDoubleSpendProofsRequest{},
			&heldDSProofStream{ctx: ctx, heldSender: held})
	}, proof, accepted, connected, disconnected))

	// The block subscriber is notified of a connection, then of a
	// disconnection.
	h := newAddrUnspentHarness(t)
	h.startDispatcher()
	h.mineBlock([]*wire.TxOut{wire.NewTxOut(50e8, p2shScript(t, otherRedeem),
		wire.TokenData{})}, nil)
	block, err := h.chain.BlockByHash(&h.prevHash)
	if err != nil {
		t.Fatalf("BlockByHash: %v", err)
	}
	blockEvents := 0
	blockChange := func() interface{} {
		blockEvents++
		if blockEvents == 1 {
			return blockEvent(block)
		}
		return &rpcEventBlockDisconnected{block}
	}
	held = newHeldSender()
	calls = append(calls, checkOtherEventsKept(t, "SubscribeBlocks", h.s, held, func() error {
		return h.s.SubscribeBlocks(&pb.SubscribeBlocksRequest{},
			&heldBlockStream{ctx: ctx, heldSender: held})
	}, blockChange, accepted, proof))

	// The calls and the stream's request reader end before the test does.
	cancel()
	for _, wait := range calls {
		wait()
	}
	waitStreamReaders(t)
}

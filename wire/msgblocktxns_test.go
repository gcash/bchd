package wire

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// TestBlockTxns tests the MsgBlockTxns API against the latest protocol
// version.
func TestBlockTxns(t *testing.T) {
	pver := ProtocolVersion
	enc := BaseEncoding

	// Ensure the command is expected value.
	wantCmd := "blocktxn"
	msg := NewMsgBlockTxns(chainhash.Hash{}, nil)
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgBlockTxns: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value.
	wantPayload := maxMessagePayload()
	maxPayload := msg.MaxPayloadLength(pver)
	if maxPayload != wantPayload {
		t.Errorf("MaxPayloadLength: wrong max payload length for "+
			"protocol version %d - got %v, want %v", pver,
			maxPayload, wantPayload)
	}

	// Test encode with latest protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, pver, enc)
	if err != nil {
		t.Errorf("encode of MsgBlockTxns failed %v err <%v>", msg,
			err)
	}

	// Older protocol versions should fail encode since message didn't
	// exist yet.
	oldPver := BIP0152Version - 1
	err = msg.BchEncode(&buf, oldPver, enc)
	if err == nil {
		s := "encode of MsgBlockTxns passed for old protocol " +
			"version %v err <%v>"
		t.Errorf(s, msg, err)
	}

	// Test decode with latest protocol version.
	readmsg := MsgSendCmpct{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err != nil {
		t.Errorf("decode of MsgBlockTxns failed [%v] err <%v>", buf,
			err)
	}

	// Older protocol versions should fail decode since message didn't
	// exist yet.
	err = readmsg.BchDecode(&buf, oldPver, enc)
	if err == nil {
		s := "decode of MsgBlockTxns passed for old protocol " +
			"version %v err <%v>"
		t.Errorf(s, msg, err)
	}
}

// TestBlockTxnsFeeFilterVersion tests the MsgBlockTxns API against the protocol
// prior to version BIP0152Version.
func TestBlockTxnsFeeFilterVersion(t *testing.T) {
	// Use the protocol version just prior to BIP0152Version changes.
	pver := BIP0152Version - 1
	enc := BaseEncoding

	msg := NewMsgBlockTxns(chainhash.Hash{}, nil)

	// Test encode with old protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, pver, enc)
	if err == nil {
		t.Errorf("encode of MsgBlockTxns succeeded when it should " +
			"have failed")
	}

	// Test decode with old protocol version.
	readmsg := MsgBlockTxns{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err == nil {
		t.Errorf("decode of MsgBlockTxns succeeded when it should " +
			"have failed")
	}
}

// TestBlockTxnsCrossProtocol tests the MsgBlockTxns API when encoding with
// the latest protocol version and decoding with FeeFilterVersion.
func TestBlockTxnsCrossProtocol(t *testing.T) {
	enc := BaseEncoding
	msg := NewMsgBlockTxns(chainhash.Hash{}, nil)

	// Encode with latest protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, ProtocolVersion, enc)
	if err != nil {
		t.Errorf("encode of MsgBlockTxns succeeded when it should failed %v err <%v>", msg,
			err)
	}

	// Decode with old protocol version.
	readmsg := MsgSendCmpct{}
	err = readmsg.BchDecode(&buf, FeeFilterVersion, enc)
	if err == nil {
		t.Errorf("decode of MsgBlockTxns failed [%v] err <%v>", buf,
			err)
	}
}

// TestBlockTxnsWire tests the GetBlockTxnsWire wire encode and decode for
// various protocol versions.
func TestBlockTxnsWire(t *testing.T) {
	msgBlockTxns := NewMsgBlockTxns(chainhash.Hash{}, []*MsgTx{blockOne.Transactions[0]})
	msgBlockTxnsEncoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Block hash
		0x01,                   // // Varint for number of transactions
		0x01, 0x00, 0x00, 0x00, // Version
		0x01, // Varint for number of transaction inputs
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Previous output hash
		0xff, 0xff, 0xff, 0xff, // Prevous output index
		0x07,                                     // Varint for length of signature script
		0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04, // Signature script (coinbase)
		0xff, 0xff, 0xff, 0xff, // Sequence
		0x01,                                           // Varint for number of transaction outputs
		0x00, 0xf2, 0x05, 0x2a, 0x01, 0x00, 0x00, 0x00, // Transaction amount
		0x43, // Varint for length of pk script
		0x41, // OP_DATA_65
		0x04, 0x96, 0xb5, 0x38, 0xe8, 0x53, 0x51, 0x9c,
		0x72, 0x6a, 0x2c, 0x91, 0xe6, 0x1e, 0xc1, 0x16,
		0x00, 0xae, 0x13, 0x90, 0x81, 0x3a, 0x62, 0x7c,
		0x66, 0xfb, 0x8b, 0xe7, 0x94, 0x7b, 0xe6, 0x3c,
		0x52, 0xda, 0x75, 0x89, 0x37, 0x95, 0x15, 0xd4,
		0xe0, 0xa6, 0x04, 0xf8, 0x14, 0x17, 0x81, 0xe6,
		0x22, 0x94, 0x72, 0x11, 0x66, 0xbf, 0x62, 0x1e,
		0x73, 0xa8, 0x2c, 0xbf, 0x23, 0x42, 0xc8, 0x58,
		0xee,                   // 65-byte uncompressed public key
		0xac,                   // OP_CHECKSIG
		0x00, 0x00, 0x00, 0x00, // Lock time
	}

	tests := []struct {
		in   *MsgBlockTxns   // Message to encode
		out  *MsgBlockTxns   // Expected decoded message
		buf  []byte          // Wire encoding
		pver uint32          // Protocol version for wire encoding
		enc  MessageEncoding // Message encoding format
	}{
		// Latest protocol version.
		{
			msgBlockTxns,
			msgBlockTxns,
			msgBlockTxnsEncoded,
			ProtocolVersion,
			BaseEncoding,
		},

		// Protocol version BIP0152Version+1
		{
			msgBlockTxns,
			msgBlockTxns,
			msgBlockTxnsEncoded,
			BIP0152Version + 1,
			BaseEncoding,
		},

		// Protocol version BIP0152Version
		{
			msgBlockTxns,
			msgBlockTxns,
			msgBlockTxnsEncoded,
			BIP0152Version,
			BaseEncoding,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode the message to wire format.
		var buf bytes.Buffer
		err := test.in.BchEncode(&buf, test.pver, test.enc)
		if err != nil {
			t.Errorf("BchEncode #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("BchEncode #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		// Decode the message from wire format.
		var msg MsgBlockTxns
		rbuf := bytes.NewReader(test.buf)
		err = msg.BchDecode(rbuf, test.pver, test.enc)
		if err != nil {
			t.Errorf("BchDecode #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&msg, test.out) {
			t.Errorf("BchDecode #%d\n got: %s want: %s", i,
				spew.Sdump(msg), spew.Sdump(test.out))
			continue
		}
	}
}

// TestMsgBlockTxns_AbsoluteIndexes tests whether map returned by the
// AbsoluteIndexes method is correct.
func TestMsgBlockTxns_AbsoluteIndexes(t *testing.T) {
	txs := make([]*MsgTx, 5)
	for i := 0; i < 5; i++ {
		tx := *multiTx
		tx.Version = int32(i)
		txs[i] = &tx
	}

	tests := []struct {
		txs             []*MsgTx
		relativeIndexes []uint32
		expectedIndexs  []uint32
	}{
		// First index starting at zero
		{
			txs,
			[]uint32{0, 2, 0, 1, 4},
			[]uint32{0, 3, 4, 6, 11},
		},
		// First index starting at non-zero
		{
			txs[:2],
			[]uint32{3, 2},
			[]uint32{3, 6},
		},
	}

	for _, test := range tests {
		btxs := NewMsgBlockTxns(chainhash.Hash{}, test.txs)
		indexMap, err := btxs.AbsoluteIndexes(test.relativeIndexes)
		if err != nil {
			t.Fatal(err)
		}
		for i, expectedIndex := range test.expectedIndexs {
			if indexMap[expectedIndex] != test.txs[i] {
				t.Errorf("Returned incorrect tx. Expected %v, got %v", test.txs[i], indexMap[expectedIndex])
			}
		}
	}
}

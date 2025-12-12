package wire

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// TestGetBlockTxns tests the MsgGetBlockTxns API against the latest protocol
// version.
func TestGetBlockTxns(t *testing.T) {
	pver := ProtocolVersion
	enc := BaseEncoding

	// Ensure the command is expected value.
	wantCmd := "getblocktxn"
	msg := NewMsgGetBlockTxns(chainhash.Hash{}, nil)
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgGetBlockTxns: wrong command - got %v want %v",
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
		t.Errorf("encode of MsgGetBlockTxns failed %v err <%v>", msg,
			err)
	}

	// Older protocol versions should fail encode since message didn't
	// exist yet.
	oldPver := BIP0152Version - 1
	err = msg.BchEncode(&buf, oldPver, enc)
	if err == nil {
		s := "encode of MsgGetBlockTxns passed for old protocol " +
			"version %v err <%v>"
		t.Errorf(s, msg, err)
	}

	// Test decode with latest protocol version.
	readmsg := MsgGetBlockTxns{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err != nil {
		t.Errorf("decode of MsgGetBlockTxns failed [%v] err <%v>", buf,
			err)
	}

	// Older protocol versions should fail decode since message didn't
	// exist yet.
	err = readmsg.BchDecode(&buf, oldPver, enc)
	if err == nil {
		s := "decode of MsgGetBlockTxns passed for old protocol " +
			"version %v err <%v>"
		t.Errorf(s, msg, err)
	}
}

// TestGetBlockTxnsFeeFilterVersion tests the MsgGetBlockTxns API against the protocol
// prior to version BIP0152Version.
func TestGetBlockTxnsFeeFilterVersion(t *testing.T) {
	// Use the protocol version just prior to BIP0152Version changes.
	pver := BIP0152Version - 1
	enc := BaseEncoding

	msg := NewMsgGetBlockTxns(chainhash.Hash{}, nil)

	// Test encode with old protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, pver, enc)
	if err == nil {
		t.Errorf("encode of MsgGetBlockTxns succeeded when it should " +
			"have failed")
	}

	// Test decode with old protocol version.
	readmsg := MsgGetBlockTxns{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err == nil {
		t.Errorf("decode of MsgGetBlockTxns succeeded when it should " +
			"have failed")
	}
}

// TestGetBlockTxnsCrossProtocol tests the MsgGetBlockTxns API when encoding with
// the latest protocol version and decoding with FeeFilterVersion.
func TestGetBlockTxnsCrossProtocol(t *testing.T) {
	enc := BaseEncoding
	msg := NewMsgGetBlockTxns(chainhash.Hash{}, nil)

	// Encode with latest protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, ProtocolVersion, enc)
	if err != nil {
		t.Errorf("encode of MsgGetBlockTxns succeeded when it should failed %v err <%v>", msg,
			err)
	}

	// Decode with old protocol version.
	readmsg := MsgGetBlockTxns{}
	err = readmsg.BchDecode(&buf, FeeFilterVersion, enc)
	if err == nil {
		t.Errorf("decode of MsgGetBlockTxns failed [%v] err <%v>", buf,
			err)
	}
}

// TestGetBlockTxnsWire tests the GetBlockTxnsWire wire encode and decode for
// various protocol versions.
func TestGetBlockTxnsWire(t *testing.T) {
	msgGetBlockTxns := NewMsgGetBlockTxns(chainhash.Hash{}, []uint32{2})
	msgGetBlockTxnsEncoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Block hash
		0x01, // Varint for number of indexes
		0x02, // Index of 2
	}

	tests := []struct {
		in   *MsgGetBlockTxns // Message to encode
		out  *MsgGetBlockTxns // Expected decoded message
		buf  []byte           // Wire encoding
		pver uint32           // Protocol version for wire encoding
		enc  MessageEncoding  // Message encoding format
	}{
		// Latest protocol version.
		{
			msgGetBlockTxns,
			msgGetBlockTxns,
			msgGetBlockTxnsEncoded,
			ProtocolVersion,
			BaseEncoding,
		},

		// Protocol version BIP0152Version+1
		{
			msgGetBlockTxns,
			msgGetBlockTxns,
			msgGetBlockTxnsEncoded,
			BIP0152Version + 1,
			BaseEncoding,
		},

		// Protocol version BIP0152Version
		{
			msgGetBlockTxns,
			msgGetBlockTxns,
			msgGetBlockTxnsEncoded,
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
		var msg MsgGetBlockTxns
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

// TestNewMsgGetBlockTxnsFromBlock tests whether the MsgGetBlockTxns is
// properly contructed by the TestNewMsgGetBlockTxnsFromBlock method.
func TestNewMsgGetBlockTxnsFromBlock(t *testing.T) {
	txs := make([]*MsgTx, 5)
	for i := 0; i < 5; i++ {
		tx := *multiTx
		tx.Version = int32(i)
		txs[i] = &tx
	}

	block1 := NewMsgBlock(&BlockHeader{})
	txsCpy := make([]*MsgTx, 5)
	copy(txsCpy, txs)
	block1.Transactions = txsCpy
	block1.Transactions[0] = nil
	block1.Transactions[2] = nil
	block1.Transactions[3] = nil

	block2 := NewMsgBlock(&BlockHeader{})
	txsCpy2 := make([]*MsgTx, 5)
	copy(txsCpy2, txs)
	block2.Transactions = txsCpy2
	block2.Transactions[2] = nil
	block2.Transactions[4] = nil

	tests := []struct {
		block           *MsgBlock
		expectedIndexes []uint32
	}{
		// First index zero
		{
			block1,
			[]uint32{0, 1, 0},
		},
		// First index non-zero
		{
			block2,
			[]uint32{2, 1},
		},
	}

	for _, test := range tests {
		gbtxns := NewMsgGetBlockTxnsFromBlock(test.block)

		if len(gbtxns.Indexes) != len(test.expectedIndexes) {
			t.Errorf("Invalid number of indexes. Expected %d got %d", len(test.expectedIndexes), len(gbtxns.Indexes))
		}

		for i, expected := range test.expectedIndexes {
			if gbtxns.Indexes[i] != expected {
				t.Errorf("Invalid index %d. Expected %d got %d", i, expected, gbtxns.Indexes[0])
			}
		}
	}
}

// TestMsgGetBlockTxns_RequestedTransactions tests whether the transactions returned
// by the RequestedTransactions method are correct.
func TestMsgGetBlockTxns_RequestedTransactions(t *testing.T) {
	tests := []struct {
		requestedIndexes []uint32
		expectedTxInexes []uint32
	}{
		// First index zero
		{
			[]uint32{0, 1, 1},
			[]uint32{0, 2, 4},
		},
		// First index non-zero
		{
			[]uint32{2, 0, 0},
			[]uint32{2, 3, 4},
		},
	}

	for _, test := range tests {
		txs := make([]*MsgTx, 5)
		for i := 0; i < 5; i++ {
			tx := *multiTx
			tx.Version = int32(i)
			txs[i] = &tx
		}
		block := NewMsgBlock(&BlockHeader{})
		block.Transactions = txs

		gbtxns := NewMsgGetBlockTxns(block.BlockHash(), test.requestedIndexes)
		rt, err := gbtxns.RequestedTransactions(block)
		if err != nil {
			t.Fatalf("Failed to create requested transactions: %s", err)
		}

		if len(rt) != len(test.expectedTxInexes) {
			t.Errorf("Invalid number of txs. Expected %d got %d", len(test.expectedTxInexes), len(rt))
		}

		for i, index := range test.expectedTxInexes {
			want := block.Transactions[index].TxHash()
			expected := rt[i].TxHash()
			if !expected.IsEqual(&want) {
				t.Errorf("Returned incorrect transaction for index %d. Expected %v got %v", i, expected, want)
			}
		}
	}
}

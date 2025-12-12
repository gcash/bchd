package wire

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// TestCmpctBlock tests the MsgCmpctBlock API.
func TestCmpctBlock(t *testing.T) {
	pver := ProtocolVersion

	// Block 1 header.
	prevHash := &blockOne.Header.PrevBlock
	merkleHash := &blockOne.Header.MerkleRoot
	bits := blockOne.Header.Bits
	nonce := blockOne.Header.Nonce
	bh := NewBlockHeader(1, prevHash, merkleHash, bits, nonce)

	// Ensure the command is expected value.
	wantCmd := "cmpctblock"
	msg := NewMsgCmpctBlock(bh)
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgCmpctBlock: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value for latest protocol version.
	// Num addresses (varInt) + max allowed addresses.
	wantPayload := maxMessagePayload()
	maxPayload := msg.MaxPayloadLength(pver)
	if maxPayload != wantPayload {
		t.Errorf("MaxPayloadLength: wrong max payload length for "+
			"protocol version %d - got %v, want %v", pver,
			maxPayload, wantPayload)
	}

	// Ensure we get the same block header data back out.
	if !reflect.DeepEqual(&msg.Header, bh) {
		t.Errorf("NewMsgCmpctBlock: wrong block header - got %v, want %v",
			spew.Sdump(&msg.Header), spew.Sdump(bh))
	}

	// Test NewMsgCmpctBlockFromBlock with no known inventory matches
	newCmpctBlock, err := NewMsgCmpctBlockFromBlock(&blockOne, nil)
	if err != nil {
		t.Fatalf("NewMsgCmpctBlockFromBlock: failed to build CmpctBlock %s", err.Error())
	}

	if len(newCmpctBlock.ShortIDs) != 0 {
		t.Errorf("NewMsgCmpctBlockFromBlock: incorrect number of short IDs - got %v want %v",
			len(newCmpctBlock.ShortIDs), 0)
	}

	if len(newCmpctBlock.PrefilledTxs) != 1 {
		t.Errorf("NewMsgCmpctBlockFromBlock: incorrect number of prefixed txs - got %v want %v",
			len(newCmpctBlock.PrefilledTxs), 1)
	}

	// Test NewMsgCmpctBlockFromBlock with a known inventory match
	ki := make(map[chainhash.Hash]bool)
	ki[blockOne.Transactions[0].TxHash()] = true
	newCmpctBlock2, err := NewMsgCmpctBlockFromBlock(&blockOne, ki)
	if err != nil {
		t.Fatalf("NewMsgCmpctBlockFromBlock: failed to build CmpctBlock %s", err.Error())
	}

	if len(newCmpctBlock2.ShortIDs) != 1 {
		t.Errorf("NewMsgCmpctBlockFromBlock: incorrect number of short IDs - got %v want %v",
			len(newCmpctBlock2.ShortIDs), 1)
	}

	if len(newCmpctBlock2.PrefilledTxs) != 0 {
		t.Errorf("NewMsgCmpctBlockFromBlock: incorrect number of prefixed txs - got %v want %v",
			len(newCmpctBlock2.PrefilledTxs), 0)
	}
}

// TestCmpctBlockHash tests the ability to generate the hash of a block accurately.
func TestCmpctBlockHash(t *testing.T) {
	// Block 1 hash.
	hashStr := "839a8e6886ab5951d76f411475428afc90947ee320161bbf18eb6048"
	wantHash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		t.Errorf("NewHashFromStr: %v", err)
	}

	// Ensure the hash produced is expected.
	msg := NewMsgCmpctBlock(&blockOne.Header)
	blockHash := msg.BlockHash()
	if !blockHash.IsEqual(wantHash) {
		t.Errorf("CmpctBlockHash: wrong hash - got %v, want %v",
			spew.Sprint(blockHash), spew.Sprint(wantHash))
	}
}

// TestCmpctBlockFeeFilterVersion tests the MsgSendCmpct API against the protocol
// prior to version SendHeadersVersion.
func TestCmpctBlockFeeFilterVersion(t *testing.T) {
	// Use the protocol version just prior to BIP0152Version changes.
	pver := BIP0152Version - 1
	enc := BaseEncoding

	msg, err := NewMsgCmpctBlockFromBlock(&blockOne, nil)
	if err != nil {
		t.Fatalf("NewMsgCmpctBlockFromBlock: failed to build CmpctBlock %s", err.Error())
	}

	// Test encode with old protocol version.
	var buf bytes.Buffer
	err = msg.BchEncode(&buf, pver, enc)
	if err == nil {
		t.Errorf("encode of MsgCmpctBlock succeeded when it should " +
			"have failed")
	}

	// Test decode with old protocol version.
	readmsg := MsgCmpctBlock{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err == nil {
		t.Errorf("decode of MsgCmpctBlock succeeded when it should " +
			"have failed")
	}
}

// TestCmpctBlockCrossProtocol tests the MsgSendCmpct API when encoding with
// the latest protocol version and decoding with FeeFilterVersion.
func TestCmpctBlockCrossProtocol(t *testing.T) {
	enc := BaseEncoding
	msg, err := NewMsgCmpctBlockFromBlock(&blockOne, nil)
	if err != nil {
		t.Fatalf("NewMsgCmpctBlockFromBlock: failed to build CmpctBlock %s", err.Error())
	}

	// Encode with latest protocol version.
	var buf bytes.Buffer
	err = msg.BchEncode(&buf, ProtocolVersion, enc)
	if err != nil {
		t.Errorf("encode of MsgCmpctBlock succeeded when it should failed %v err <%v>", msg,
			err)
	}

	// Decode with old protocol version.
	readmsg := MsgCmpctBlock{}
	err = readmsg.BchDecode(&buf, FeeFilterVersion, enc)
	if err == nil {
		t.Errorf("decode of MsgCmpctBlock failed [%v] err <%v>", buf,
			err)
	}
}

// TestCmpctBlockWire tests the MsgCmpctBlock wire encode and decode for various numbers
// of transaction inputs and outputs and protocol versions.
func TestCmpctBlockWire(t *testing.T) {
	tests := []struct {
		in   *MsgCmpctBlock  // Message to encode
		out  *MsgCmpctBlock  // Expected decoded message
		buf  []byte          // Wire encoding
		pver uint32          // Protocol version for wire encoding
		enc  MessageEncoding // Message encoding format
	}{
		// Latest protocol version.
		{
			&cmpctBlockOne,
			&cmpctBlockOne,
			cmpctBlockOneBytes,
			ProtocolVersion,
			BaseEncoding,
		},

		// Protocol version BIP0152Version+1
		{
			&cmpctBlockOne,
			&cmpctBlockOne,
			cmpctBlockOneBytes,
			BIP0152Version + 1,
			BaseEncoding,
		},

		// Protocol version BIP0152Version
		{
			&cmpctBlockOne,
			&cmpctBlockOne,
			cmpctBlockOneBytes,
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
		var msg MsgCmpctBlock
		rbuf := bytes.NewReader(test.buf)
		err = msg.BchDecode(rbuf, test.pver, test.enc)
		if err != nil {
			t.Errorf("BchDecode #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&msg, test.out) {
			t.Errorf("BchDecode #%d\n got: %s want: %s", i,
				spew.Sdump(&msg), spew.Sdump(test.out))
			continue
		}
	}
}

// cmpctBlockOne is the first block in the mainnet block chain.
var cmpctBlockOne = MsgCmpctBlock{
	Header: blockOne.Header,
	Nonce:  0,
	ShortIDs: [][6]byte{
		{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
	},
	PrefilledTxs: []*PrefilledTx{
		{
			Index: 0,
			Tx:    blockOne.Transactions[0],
		},
	},
}

// cmpctBlockOne serialized bytes.
var cmpctBlockOneBytes = []byte{
	0x01, 0x00, 0x00, 0x00, // Version 1
	0x6f, 0xe2, 0x8c, 0x0a, 0xb6, 0xf1, 0xb3, 0x72,
	0xc1, 0xa6, 0xa2, 0x46, 0xae, 0x63, 0xf7, 0x4f,
	0x93, 0x1e, 0x83, 0x65, 0xe1, 0x5a, 0x08, 0x9c,
	0x68, 0xd6, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00, // PrevBlock
	0x98, 0x20, 0x51, 0xfd, 0x1e, 0x4b, 0xa7, 0x44,
	0xbb, 0xbe, 0x68, 0x0e, 0x1f, 0xee, 0x14, 0x67,
	0x7b, 0xa1, 0xa3, 0xc3, 0x54, 0x0b, 0xf7, 0xb1,
	0xcd, 0xb6, 0x06, 0xe8, 0x57, 0x23, 0x3e, 0x0e, // MerkleRoot
	0x61, 0xbc, 0x66, 0x49, // Timestamp
	0xff, 0xff, 0x00, 0x1d, // Bits
	0x01, 0xe3, 0x62, 0x99, // Nonce
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Nonce
	0x01,                               // Varint for number of short IDs
	0x00, 0x11, 0x22, 0x33, 0x44, 0x55, // First short ID
	0x01,                   // Varint for number of prefilled transactions
	0x00,                   // Varint for index of transaction
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

func TestMsgCmpctBlock_TotalTransactions(t *testing.T) {
	total := cmpctBlockOne.TotalTransactions()
	if total != 2 {
		t.Errorf("MsgCmpctBlock: incorrect total transactions - got %v want %v",
			total, 2)
	}
}

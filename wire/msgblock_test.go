// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"io"
	"math/rand"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// TestBlock tests the MsgBlock API.
func TestBlock(t *testing.T) {
	pver := ProtocolVersion

	// Block 1 header.
	prevHash := &blockOne.Header.PrevBlock
	merkleHash := &blockOne.Header.MerkleRoot
	bits := blockOne.Header.Bits
	nonce := blockOne.Header.Nonce
	bh := NewBlockHeader(1, prevHash, merkleHash, bits, nonce)

	// Ensure the command is expected value.
	wantCmd := "block"
	msg := NewMsgBlock(bh)
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgBlock: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value for latest protocol version.
	// Num addresses (varInt) + max allowed addresses.
	// Using "* 4" is not the most optimal approach.
	// A cleaner solution would involve modifying the behavior of wire.SetLimits.
	wantPayload := fixedExcessiveBlockSize * 4
	maxPayload := msg.MaxPayloadLength(pver)
	if maxPayload != wantPayload {
		t.Errorf("MaxPayloadLength: wrong max payload length for "+
			"protocol version %d - got %v, want %v", pver,
			maxPayload, wantPayload)
	}

	// Ensure we get the same block header data back out.
	if !reflect.DeepEqual(&msg.Header, bh) {
		t.Errorf("NewMsgBlock: wrong block header - got %v, want %v",
			spew.Sdump(&msg.Header), spew.Sdump(bh))
	}

	// Ensure transactions are added properly.
	tx := blockOne.Transactions[0].Copy()
	_ = msg.AddTransaction(tx)
	if !reflect.DeepEqual(msg.Transactions, blockOne.Transactions) {
		t.Errorf("AddTransaction: wrong transactions - got %v, want %v",
			spew.Sdump(msg.Transactions),
			spew.Sdump(blockOne.Transactions))
	}

	// Ensure transactions are properly cleared.
	msg.ClearTransactions()
	if len(msg.Transactions) != 0 {
		t.Errorf("ClearTransactions: wrong transactions - got %v, want %v",
			len(msg.Transactions), 0)
	}
}

// TestBlockTxHashes tests the ability to generate a slice of all transaction
// hashes from a block accurately.
func TestBlockTxHashes(t *testing.T) {
	// Block 1, transaction 1 hash.
	hashStr := "0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098"
	wantHash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		t.Errorf("NewHashFromStr: %v", err)
		return
	}

	wantHashes := []chainhash.Hash{*wantHash}
	hashes, err := blockOne.TxHashes()
	if err != nil {
		t.Errorf("TxHashes: %v", err)
	}
	if !reflect.DeepEqual(hashes, wantHashes) {
		t.Errorf("TxHashes: wrong transaction hashes - got %v, want %v",
			spew.Sdump(hashes), spew.Sdump(wantHashes))
	}
}

// TestBlockHash tests the ability to generate the hash of a block accurately.
func TestBlockHash(t *testing.T) {
	// Block 1 hash.
	hashStr := "839a8e6886ab5951d76f411475428afc90947ee320161bbf18eb6048"
	wantHash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		t.Errorf("NewHashFromStr: %v", err)
	}

	// Ensure the hash produced is expected.
	blockHash := blockOne.BlockHash()
	if !blockHash.IsEqual(wantHash) {
		t.Errorf("BlockHash: wrong hash - got %v, want %v",
			spew.Sprint(blockHash), spew.Sprint(wantHash))
	}
}

// TestBlockWire tests the MsgBlock wire encode and decode for various numbers
// of transaction inputs and outputs and protocol versions.
func TestBlockWire(t *testing.T) {
	tests := []struct {
		in     *MsgBlock       // Message to encode
		out    *MsgBlock       // Expected decoded message
		buf    []byte          // Wire encoding
		txLocs []TxLoc         // Expected transaction locations
		pver   uint32          // Protocol version for wire encoding
		enc    MessageEncoding // Message encoding format
	}{
		// Latest protocol version.
		{
			&blockOne,
			&blockOne,
			blockOneBytes,
			blockOneTxLocs,
			ProtocolVersion,
			BaseEncoding,
		},

		// Protocol version BIP0035Version.
		{
			&blockOne,
			&blockOne,
			blockOneBytes,
			blockOneTxLocs,
			BIP0035Version,
			BaseEncoding,
		},

		// Protocol version BIP0031Version.
		{
			&blockOne,
			&blockOne,
			blockOneBytes,
			blockOneTxLocs,
			BIP0031Version,
			BaseEncoding,
		},

		// Protocol version NetAddressTimeVersion.
		{
			&blockOne,
			&blockOne,
			blockOneBytes,
			blockOneTxLocs,
			NetAddressTimeVersion,
			BaseEncoding,
		},

		// Protocol version MultipleAddressVersion.
		{
			&blockOne,
			&blockOne,
			blockOneBytes,
			blockOneTxLocs,
			MultipleAddressVersion,
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
		var msg MsgBlock
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

// TestBlockWireErrors performs negative tests against wire encode and decode
// of MsgBlock to confirm error paths work correctly.
func TestBlockWireErrors(t *testing.T) {
	// Use protocol version 60002 specifically here instead of the latest
	// because the test data is using bytes encoded with that protocol
	// version.
	pver := uint32(60002)

	tests := []struct {
		in       *MsgBlock       // Value to encode
		buf      []byte          // Wire encoding
		pver     uint32          // Protocol version for wire encoding
		enc      MessageEncoding // Message encoding format
		max      int             // Max size of fixed buffer to induce errors
		writeErr error           // Expected write error
		readErr  error           // Expected read error
	}{
		// Force error in version.
		{&blockOne, blockOneBytes, pver, BaseEncoding, 0, io.ErrShortWrite, io.EOF},
		// Force error in prev block hash.
		{&blockOne, blockOneBytes, pver, BaseEncoding, 4, io.ErrShortWrite, io.EOF},
		// Force error in merkle root.
		{&blockOne, blockOneBytes, pver, BaseEncoding, 36, io.ErrShortWrite, io.EOF},
		// Force error in timestamp.
		{&blockOne, blockOneBytes, pver, BaseEncoding, 68, io.ErrShortWrite, io.EOF},
		// Force error in difficulty bits.
		{&blockOne, blockOneBytes, pver, BaseEncoding, 72, io.ErrShortWrite, io.EOF},
		// Force error in header nonce.
		{&blockOne, blockOneBytes, pver, BaseEncoding, 76, io.ErrShortWrite, io.EOF},
		// Force error in transaction count.
		{&blockOne, blockOneBytes, pver, BaseEncoding, 80, io.ErrShortWrite, io.EOF},
		// Force error in transactions.
		{&blockOne, blockOneBytes, pver, BaseEncoding, 81, io.ErrShortWrite, io.EOF},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode to wire format.
		w := newFixedWriter(test.max)
		err := test.in.BchEncode(w, test.pver, test.enc)
		if err != test.writeErr {
			t.Errorf("BchEncode #%d wrong error got: %v, want: %v",
				i, err, test.writeErr)
			continue
		}

		// Decode from wire format.
		var msg MsgBlock
		r := newFixedReader(test.max, test.buf)
		err = msg.BchDecode(r, test.pver, test.enc)
		if err != test.readErr {
			t.Errorf("BchDecode #%d wrong error got: %v, want: %v",
				i, err, test.readErr)
			continue
		}
	}
}

// TestBlockSerialize tests MsgBlock serialize and deserialize.
func TestBlockSerialize(t *testing.T) {
	tests := []struct {
		in     *MsgBlock // Message to encode
		out    *MsgBlock // Expected decoded message
		buf    []byte    // Serialized data
		txLocs []TxLoc   // Expected transaction locations
	}{
		{
			&blockOne,
			&blockOne,
			blockOneBytes,
			blockOneTxLocs,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Serialize the block.
		var buf bytes.Buffer
		err := test.in.Serialize(&buf)
		if err != nil {
			t.Errorf("Serialize #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("Serialize #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		// Deserialize the block.
		var block MsgBlock
		rbuf := bytes.NewReader(test.buf)
		err = block.Deserialize(rbuf)
		if err != nil {
			t.Errorf("Deserialize #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&block, test.out) {
			t.Errorf("Deserialize #%d\n got: %s want: %s", i,
				spew.Sdump(&block), spew.Sdump(test.out))
			continue
		}

		// Deserialize the block while gathering transaction location
		// information.
		var txLocBlock MsgBlock
		br := bytes.NewBuffer(test.buf)
		txLocs, err := txLocBlock.DeserializeTxLoc(br)
		if err != nil {
			t.Errorf("DeserializeTxLoc #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&txLocBlock, test.out) {
			t.Errorf("DeserializeTxLoc #%d\n got: %s want: %s", i,
				spew.Sdump(&txLocBlock), spew.Sdump(test.out))
			continue
		}
		if !reflect.DeepEqual(txLocs, test.txLocs) {
			t.Errorf("DeserializeTxLoc #%d\n got: %s want: %s", i,
				spew.Sdump(txLocs), spew.Sdump(test.txLocs))
			continue
		}

		// Find the transaction locations without decoding the block.
		txLocs, err = BlockTxLocs(test.buf)
		if err != nil {
			t.Errorf("BlockTxLocs #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(txLocs, test.txLocs) {
			t.Errorf("BlockTxLocs #%d\n got: %s want: %s", i,
				spew.Sdump(txLocs), spew.Sdump(test.txLocs))
			continue
		}
	}
}

// TestBlockSerializeErrors performs negative tests against wire encode and
// decode of MsgBlock to confirm error paths work correctly.
func TestBlockSerializeErrors(t *testing.T) {
	tests := []struct {
		in       *MsgBlock // Value to encode
		buf      []byte    // Serialized data
		max      int       // Max size of fixed buffer to induce errors
		writeErr error     // Expected write error
		readErr  error     // Expected read error
	}{
		// Force error in version.
		{&blockOne, blockOneBytes, 0, io.ErrShortWrite, io.EOF},
		// Force error in prev block hash.
		{&blockOne, blockOneBytes, 4, io.ErrShortWrite, io.EOF},
		// Force error in merkle root.
		{&blockOne, blockOneBytes, 36, io.ErrShortWrite, io.EOF},
		// Force error in timestamp.
		{&blockOne, blockOneBytes, 68, io.ErrShortWrite, io.EOF},
		// Force error in difficulty bits.
		{&blockOne, blockOneBytes, 72, io.ErrShortWrite, io.EOF},
		// Force error in header nonce.
		{&blockOne, blockOneBytes, 76, io.ErrShortWrite, io.EOF},
		// Force error in transaction count.
		{&blockOne, blockOneBytes, 80, io.ErrShortWrite, io.EOF},
		// Force error in transactions.
		{&blockOne, blockOneBytes, 81, io.ErrShortWrite, io.EOF},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Serialize the block.
		w := newFixedWriter(test.max)
		err := test.in.Serialize(w)
		if err != test.writeErr {
			t.Errorf("Serialize #%d wrong error got: %v, want: %v",
				i, err, test.writeErr)
			continue
		}

		// Deserialize the block.
		var block MsgBlock
		r := newFixedReader(test.max, test.buf)
		err = block.Deserialize(r)
		if err != test.readErr {
			t.Errorf("Deserialize #%d wrong error got: %v, want: %v",
				i, err, test.readErr)
			continue
		}

		var txLocBlock MsgBlock
		br := bytes.NewBuffer(test.buf[0:test.max])
		_, err = txLocBlock.DeserializeTxLoc(br)
		if err != test.readErr {
			t.Errorf("DeserializeTxLoc #%d wrong error got: %v, want: %v",
				i, err, test.readErr)
			continue
		}

		if _, err := BlockTxLocs(test.buf[0:test.max]); err == nil {
			t.Errorf("BlockTxLocs #%d did not fail", i)
			continue
		}
	}
}

// TestBlockOverflowErrors  performs tests to ensure deserializing blocks which
// are intentionally crafted to use large values for the number of transactions
// are handled properly.  This could otherwise potentially be used as an attack
// vector.
func TestBlockOverflowErrors(t *testing.T) {
	// Use protocol version 70001 specifically here instead of the latest
	// protocol version because the test data is using bytes encoded with
	// that version.
	pver := uint32(70001)

	tests := []struct {
		buf  []byte          // Wire encoding
		pver uint32          // Protocol version for wire encoding
		enc  MessageEncoding // Message encoding format
		err  error           // Expected error
	}{
		// Block that claims to have ~uint64(0) transactions.
		{
			[]byte{
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
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, // TxnCount
			}, pver, BaseEncoding, &MessageError{},
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Decode from wire format.
		var msg MsgBlock
		r := bytes.NewReader(test.buf)
		err := msg.BchDecode(r, test.pver, test.enc)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("BchDecode #%d wrong error got: %v, want: %v",
				i, err, reflect.TypeOf(test.err))
			continue
		}

		// Deserialize from wire format.
		r = bytes.NewReader(test.buf)
		err = msg.Deserialize(r)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("Deserialize #%d wrong error got: %v, want: %v",
				i, err, reflect.TypeOf(test.err))
			continue
		}

		// Deserialize with transaction location info from wire format.
		br := bytes.NewBuffer(test.buf)
		_, err = msg.DeserializeTxLoc(br)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("DeserializeTxLoc #%d wrong error got: %v, "+
				"want: %v", i, err, reflect.TypeOf(test.err))
			continue
		}

		// Find the transaction locations from wire format.
		_, err = BlockTxLocs(test.buf)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("BlockTxLocs #%d wrong error got: %v, "+
				"want: %v", i, err, reflect.TypeOf(test.err))
			continue
		}
	}
}

// TestBlockSerializeSize performs tests to ensure the serialize size for
// various blocks is accurate.
func TestBlockSerializeSize(t *testing.T) {
	// Block with no transactions.
	noTxBlock := NewMsgBlock(&blockOne.Header)

	tests := []struct {
		in   *MsgBlock // Block to encode
		size int       // Expected serialized size
	}{
		// Block with no transactions.
		{noTxBlock, 81},

		// First block in the mainnet block chain.
		{&blockOne, len(blockOneBytes)},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		serializedSize := test.in.SerializeSize()
		if serializedSize != test.size {
			t.Errorf("MsgBlock.SerializeSize: #%d got: %d, want: "+
				"%d", i, serializedSize, test.size)
			continue
		}
	}
}

// blockOne is the first block in the mainnet block chain.
var blockOne = MsgBlock{
	Header: BlockHeader{
		Version: 1,
		PrevBlock: chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
			0x6f, 0xe2, 0x8c, 0x0a, 0xb6, 0xf1, 0xb3, 0x72,
			0xc1, 0xa6, 0xa2, 0x46, 0xae, 0x63, 0xf7, 0x4f,
			0x93, 0x1e, 0x83, 0x65, 0xe1, 0x5a, 0x08, 0x9c,
			0x68, 0xd6, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00,
		}),
		MerkleRoot: chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
			0x98, 0x20, 0x51, 0xfd, 0x1e, 0x4b, 0xa7, 0x44,
			0xbb, 0xbe, 0x68, 0x0e, 0x1f, 0xee, 0x14, 0x67,
			0x7b, 0xa1, 0xa3, 0xc3, 0x54, 0x0b, 0xf7, 0xb1,
			0xcd, 0xb6, 0x06, 0xe8, 0x57, 0x23, 0x3e, 0x0e,
		}),

		Timestamp: time.Unix(0x4966bc61, 0), // 2009-01-08 20:54:25 -0600 CST
		Bits:      0x1d00ffff,               // 486604799
		Nonce:     0x9962e301,               // 2573394689
	},
	Transactions: []*MsgTx{
		{
			Version: 1,
			TxIn: []*TxIn{
				{
					PreviousOutPoint: OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{
						0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04,
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*TxOut{
				{
					Value: 0x12a05f200,
					PkScript: []byte{
						0x41, // OP_DATA_65
						0x04, 0x96, 0xb5, 0x38, 0xe8, 0x53, 0x51, 0x9c,
						0x72, 0x6a, 0x2c, 0x91, 0xe6, 0x1e, 0xc1, 0x16,
						0x00, 0xae, 0x13, 0x90, 0x81, 0x3a, 0x62, 0x7c,
						0x66, 0xfb, 0x8b, 0xe7, 0x94, 0x7b, 0xe6, 0x3c,
						0x52, 0xda, 0x75, 0x89, 0x37, 0x95, 0x15, 0xd4,
						0xe0, 0xa6, 0x04, 0xf8, 0x14, 0x17, 0x81, 0xe6,
						0x22, 0x94, 0x72, 0x11, 0x66, 0xbf, 0x62, 0x1e,
						0x73, 0xa8, 0x2c, 0xbf, 0x23, 0x42, 0xc8, 0x58,
						0xee, // 65-byte signature
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
	},
}

// Block one serialized bytes.
var blockOneBytes = []byte{
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
	0x01,                   // TxnCount
	0x01, 0x00, 0x00, 0x00, // Version
	0x01, // Varint for number of transaction inputs
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Previous output hash
	0xff, 0xff, 0xff, 0xff, // Previous output index
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

// Transaction location information for block one transactions.
var blockOneTxLocs = []TxLoc{
	{TxStart: 81, TxLen: 134},
}

// TestBlockTxCountAlloc ensures the memory allocated while decoding a block
// grows with the transactions actually read rather than with the transaction
// count claimed by the block, for each of the block decoding functions.
func TestBlockTxCountAlloc(t *testing.T) {
	// withTxCount returns a block encoding with the given transaction count
	// and no transactions after it.
	withTxCount := func(count uint64) []byte {
		var buf bytes.Buffer
		_ = writeBlockHeader(&buf, 0, &blockOne.Header)
		_ = WriteVarInt(&buf, 0, count)
		return buf.Bytes()
	}

	tests := []struct {
		name string // Test description
		buf  []byte // Wire encoding
		err  error  // Expected error
	}{
		{
			name: "count at max with no transactions",
			buf:  withTxCount(uint64(maxTxPerBlock())),
			err:  io.EOF,
		},
		{
			name: "count one above max",
			buf:  withTxCount(uint64(maxTxPerBlock()) + 1),
			err:  &MessageError{},
		},
	}

	decoders := []struct {
		name   string
		decode func(buf []byte) error
	}{
		{"BchDecode", func(buf []byte) error {
			var msg MsgBlock
			return msg.BchDecode(bytes.NewReader(buf),
				ProtocolVersion, BaseEncoding)
		}},
		{"Deserialize", func(buf []byte) error {
			var msg MsgBlock
			return msg.Deserialize(bytes.NewReader(buf))
		}},
		{"DeserializeTxLoc", func(buf []byte) error {
			var msg MsgBlock
			_, err := msg.DeserializeTxLoc(bytes.NewBuffer(buf))
			return err
		}},
		{"BlockTxLocs", func(buf []byte) error {
			_, err := BlockTxLocs(buf)
			return err
		}},
	}

	t.Logf("Running %d tests", len(tests)*len(decoders))
	for _, test := range tests {
		for _, d := range decoders {
			var err error
			alloc := decodeAlloc(func() {
				err = d.decode(test.buf)
			})
			if alloc > maxBoundedDecodeAlloc {
				t.Errorf("%s %s: allocated %d bytes, want at "+
					"most %d", d.name, test.name, alloc,
					maxBoundedDecodeAlloc)
			}
			if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
				t.Errorf("%s %s: wrong error got: %v, want: %v",
					d.name, test.name, err,
					reflect.TypeOf(test.err))
			}
		}
	}
}

// randTxLocBlock returns a serialized block of random transactions whose
// counts and scripts use every length encoding, with outputs that hold token
// data, outputs that don't, and outputs whose scripts only start like token
// data.
func randTxLocBlock(r *rand.Rand) []byte {
	randBytes := func(n int) []byte {
		b := make([]byte, n)
		r.Read(b)
		return b
	}
	count := func() int {
		if r.Intn(30) == 0 {
			return 0xfd + r.Intn(3)
		}
		return r.Intn(4)
	}
	scriptLen := func() int {
		switch r.Intn(200) {
		case 0:
			return 0x10000 + r.Intn(10)
		case 1, 2, 3, 4:
			return 0xfd + r.Intn(300)
		}
		return r.Intn(40)
	}

	block := MsgBlock{Header: blockOne.Header}
	for range 1 + r.Intn(10) {
		tx := NewMsgTx(r.Int31())
		for range count() {
			tx.AddTxIn(&TxIn{
				PreviousOutPoint: OutPoint{Index: r.Uint32()},
				SignatureScript:  randBytes(scriptLen()),
				Sequence:         r.Uint32(),
			})
		}
		for range count() {
			out := &TxOut{Value: r.Int63(), PkScript: randBytes(scriptLen())}
			switch r.Intn(4) {
			case 0:
				out.TokenData = TokenData{BitField: HAS_AMOUNT,
					Amount: 1 + r.Uint64()>>1}
				r.Read(out.TokenData.CategoryID[:])
			case 1:
				out.TokenData = TokenData{
					BitField:   HAS_NFT | HAS_COMMITMENT_LENGTH | MINTING,
					Commitment: randBytes(1 + r.Intn(40)),
				}
			case 2:
				out.PkScript = append([]byte{PREFIX_BYTE}, out.PkScript...)
			}
			tx.AddTxOut(out)
		}
		tx.LockTime = r.Uint32()
		_ = block.AddTransaction(tx)
	}
	var buf bytes.Buffer
	if err := block.Serialize(&buf); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// deserializeTxLocs returns the transaction locations DeserializeTxLoc finds
// in the passed serialized block.
func deserializeTxLocs(buf []byte) ([]TxLoc, error) {
	var msg MsgBlock
	return msg.DeserializeTxLoc(bytes.NewBuffer(buf))
}

// TestBlockTxLocsMatchesDeserialize ensures BlockTxLocs finds the same
// transaction locations as DeserializeTxLoc in random blocks, and in damaged
// copies of them fails where DeserializeTxLoc fails.
func TestBlockTxLocsMatchesDeserialize(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(1))
	for i := range 300 {
		buf := randTxLocBlock(r)
		want, err := deserializeTxLocs(buf)
		if err != nil {
			t.Fatalf("block %d: DeserializeTxLoc: %v", i, err)
		}
		got, err := BlockTxLocs(buf)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("block %d: BlockTxLocs = %v, %v, want %v", i, got,
				err, want)
		}

		for j := range 20 {
			damaged := slices.Clone(buf)
			k := r.Intn(len(damaged))
			switch r.Intn(3) {
			case 0:
				damaged = damaged[:k]
			case 1:
				damaged[k] = byte(r.Intn(256))
			case 2:
				damaged = slices.Insert(damaged, k, byte(r.Intn(256)))
			}
			want, wantErr := deserializeTxLocs(damaged)
			got, err := BlockTxLocs(damaged)
			if reflect.TypeOf(err) != reflect.TypeOf(wantErr) ||
				!reflect.DeepEqual(got, want) {

				t.Fatalf("block %d damage %d: BlockTxLocs = %v, %v, "+
					"want %v, %v", i, j, got, err, want, wantErr)
			}
		}
	}
}

// TestBlockTxLocsLimits ensures BlockTxLocs fails, as DeserializeTxLoc
// does, on a transaction whose input or output count or script length is over
// the limit for it, and not on one at the limit.
func TestBlockTxLocsLimits(t *testing.T) {
	// block returns a serialized block of one transaction that starts with
	// the passed fields, and is cut off after them.
	block := func(fields ...any) []byte {
		var buf bytes.Buffer
		_ = writeBlockHeader(&buf, 0, &blockOne.Header)
		_ = WriteVarInt(&buf, 0, 1)
		buf.Write([]byte{1, 0, 0, 0})
		for _, f := range fields {
			switch f := f.(type) {
			case uint32:
				_ = WriteVarInt(&buf, 0, uint64(f))
			case []byte:
				buf.Write(f)
			}
		}
		return buf.Bytes()
	}
	outpoint := make([]byte, chainhash.HashSize+4)
	value := make([]byte, 8)
	tests := []struct {
		name string
		buf  []byte
		err  error
	}{
		{"inputs at max", block(maxTxInPerMessage()), io.EOF},
		{"inputs over max", block(maxTxInPerMessage() + 1), &MessageError{}},
		{"signature script at max",
			block(uint32(1), outpoint, maxTxInPerMessage()), io.EOF},
		{"signature script over max",
			block(uint32(1), outpoint, maxTxInPerMessage()+1),
			&MessageError{}},
		{"outputs at max", block(uint32(0), maxTxOutPerMessage()), io.EOF},
		{"outputs over max", block(uint32(0), maxTxOutPerMessage()+1),
			&MessageError{}},
		{"public key script at max",
			block(uint32(0), uint32(1), value, maxMessagePayload()), io.EOF},
		{"public key script over max",
			block(uint32(0), uint32(1), value, maxMessagePayload()+1),
			&MessageError{}},
	}
	for _, test := range tests {
		if _, err := deserializeTxLocs(test.buf); reflect.TypeOf(err) !=
			reflect.TypeOf(test.err) {

			t.Errorf("%s: DeserializeTxLoc error %v, want %T", test.name,
				err, test.err)
		}
		if _, err := BlockTxLocs(test.buf); reflect.TypeOf(err) !=
			reflect.TypeOf(test.err) {

			t.Errorf("%s: BlockTxLocs error %v, want %T", test.name, err,
				test.err)
		}
	}
}

// FuzzBlockTxLocs ensures BlockTxLocs finds the same transaction locations
// as DeserializeTxLoc, and fails where it fails.
func FuzzBlockTxLocs(f *testing.F) {
	f.Add(blockOneBytes)
	r := rand.New(rand.NewSource(2))
	for range 4 {
		f.Add(randTxLocBlock(r))
	}
	f.Fuzz(func(t *testing.T, buf []byte) {
		want, wantErr := deserializeTxLocs(buf)
		got, err := BlockTxLocs(buf)
		if reflect.TypeOf(err) != reflect.TypeOf(wantErr) ||
			!reflect.DeepEqual(got, want) {
			t.Fatalf("BlockTxLocs = %v, %v, want %v, %v", got, err, want,
				wantErr)
		}
	})
}

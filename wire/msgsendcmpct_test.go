package wire

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

// TestSendCmpct tests the MsgSendCmpct API against the latest protocol
// version.
func TestSendCmpct(t *testing.T) {
	pver := ProtocolVersion
	enc := BaseEncoding

	// Ensure the command is expected value.
	wantCmd := "sendcmpct"
	msg := NewMsgSendCmpct(true, CompactBlocksProtocolVersion)
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgSendCmpct: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value.
	wantPayload := uint32(9)
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
		t.Errorf("encode of MsgSendCmpct failed %v err <%v>", msg,
			err)
	}

	// Older protocol versions should fail encode since message didn't
	// exist yet.
	oldPver := BIP0152Version - 1
	err = msg.BchEncode(&buf, oldPver, enc)
	if err == nil {
		s := "encode of MsgSendCmpct passed for old protocol " +
			"version %v err <%v>"
		t.Errorf(s, msg, err)
	}

	// Test decode with latest protocol version.
	readmsg := MsgSendCmpct{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err != nil {
		t.Errorf("decode of MsgSendCmpct failed [%v] err <%v>", buf,
			err)
	}

	// Older protocol versions should fail decode since message didn't
	// exist yet.
	err = readmsg.BchDecode(&buf, oldPver, enc)
	if err == nil {
		s := "decode of MsgSendCmpct passed for old protocol " +
			"version %v err <%v>"
		t.Errorf(s, msg, err)
	}
}

// TestSendCmpctFeeFilterVersion tests the MsgSendCmpct API against the protocol
// prior to version BIP0152Version.
func TestSendCmpctFeeFilterVersion(t *testing.T) {
	// Use the protocol version just prior to BIP0152Version changes.
	pver := BIP0152Version - 1
	enc := BaseEncoding

	msg := NewMsgSendCmpct(true, CompactBlocksProtocolVersion)

	// Test encode with old protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, pver, enc)
	if err == nil {
		t.Errorf("encode of MsgSendCmpct succeeded when it should " +
			"have failed")
	}

	// Test decode with old protocol version.
	readmsg := MsgSendCmpct{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err == nil {
		t.Errorf("decode of MsgSendCmpct succeeded when it should " +
			"have failed")
	}
}

// TestSendCmpctCrossProtocol tests the MsgSendCmpct API when encoding with
// the latest protocol version and decoding with FeeFilterVersion.
func TestSendCmpctCrossProtocol(t *testing.T) {
	enc := BaseEncoding
	msg := NewMsgSendCmpct(true, CompactBlocksProtocolVersion)

	// Encode with latest protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, ProtocolVersion, enc)
	if err != nil {
		t.Errorf("encode of MsgSendCmpct succeeded when it should failed %v err <%v>", msg,
			err)
	}

	// Decode with old protocol version.
	readmsg := MsgSendCmpct{}
	err = readmsg.BchDecode(&buf, FeeFilterVersion, enc)
	if err == nil {
		t.Errorf("decode of MsgSendCmpct failed [%v] err <%v>", buf,
			err)
	}
}

// TestSendCmpctWire tests the MsgSendCmpct wire encode and decode for
// various protocol versions.
func TestSendCmpctWire(t *testing.T) {
	msgSendCmpct := NewMsgSendCmpct(true, CompactBlocksProtocolVersion)
	msgSendCmpctEncoded := []byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	tests := []struct {
		in   *MsgSendCmpct   // Message to encode
		out  *MsgSendCmpct   // Expected decoded message
		buf  []byte          // Wire encoding
		pver uint32          // Protocol version for wire encoding
		enc  MessageEncoding // Message encoding format
	}{
		// Latest protocol version.
		{
			msgSendCmpct,
			msgSendCmpct,
			msgSendCmpctEncoded,
			ProtocolVersion,
			BaseEncoding,
		},

		// Protocol version BIP0152Version+1
		{
			msgSendCmpct,
			msgSendCmpct,
			msgSendCmpctEncoded,
			BIP0152Version + 1,
			BaseEncoding,
		},

		// Protocol version BIP0152Version
		{
			msgSendCmpct,
			msgSendCmpct,
			msgSendCmpctEncoded,
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
		var msg MsgSendCmpct
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

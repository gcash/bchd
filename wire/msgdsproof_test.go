package wire

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"reflect"
	"testing"
)

// TestDSProof tests the MsgDSProof API against the latest protocol
// version.
func TestDSProof(t *testing.T) {
	pver := ProtocolVersion
	enc := BaseEncoding

	// Ensure the command is expected value.
	wantCmd := "dsproof-beta"
	msg := NewMsgDSProof(OutPoint{}, Spender{}, Spender{})
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgDSProof: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value.
	wantPayload := uint32(20270)
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
		t.Errorf("encode of MsgDSProof failed %v err <%v>", msg,
			err)
	}

	// Older protocol versions should fail encode since message didn't
	// exist yet.
	oldPver := DoubleSpendProofVersion - 1
	err = msg.BchEncode(&buf, oldPver, enc)
	if err == nil {
		s := "encode of MsgDSProof passed for old protocol " +
			"version %v err <%v>"
		t.Errorf(s, msg, err)
	}

	// Test decode with latest protocol version.
	readmsg := MsgDSProof{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err != nil {
		t.Errorf("decode of MsgDSProof failed [%v] err <%v>", buf,
			err)
	}

	// Older protocol versions should fail decode since message didn't
	// exist yet.
	err = readmsg.BchDecode(&buf, oldPver, enc)
	if err == nil {
		s := "decode of MsgDSProof passed for old protocol " +
			"version %v err <%v>"
		t.Errorf(s, msg, err)
	}
}

// TestDSProofVersion tests the MsgDSProof API against the protocol
// prior to version NoValidationRelayVersion.
func TestDSProofVersion(t *testing.T) {
	// Use the protocol version just prior to BIP0152Version changes.
	pver := NoValidationRelayVersion - 1
	enc := BaseEncoding

	msg := NewMsgDSProof(OutPoint{}, Spender{}, Spender{})

	// Test encode with old protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, pver, enc)
	if err == nil {
		t.Errorf("encode of MsgDSProof succeeded when it should " +
			"have failed")
	}

	// Test decode with old protocol version.
	readmsg := MsgDSProof{}
	err = readmsg.BchDecode(&buf, pver, enc)
	if err == nil {
		t.Errorf("decode of MsgDSProof succeeded when it should " +
			"have failed")
	}
}

// TestDSProofCrossProtocol tests the MsgDSProof API when encoding with
// the latest protocol version and decoding with BIP0152Version.
func TestDSProofCrossProtocol(t *testing.T) {
	enc := BaseEncoding
	msg := NewMsgDSProof(OutPoint{}, Spender{}, Spender{})

	// Encode with latest protocol version.
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, ProtocolVersion, enc)
	if err != nil {
		t.Errorf("encode of MsgDSProof succeeded when it should failed %v err <%v>", msg,
			err)
	}
	for _, b := range buf.Bytes() {
		fmt.Print("0x" + hex.EncodeToString([]byte{b}) + ", ")
	}

	// Decode with old protocol version.
	readmsg := MsgDSProof{}
	err = readmsg.BchDecode(&buf, BIP0152Version, enc)
	if err == nil {
		t.Errorf("decode of MsgDSProof failed [%v] err <%v>", buf,
			err)
	}
}

// TestDSProofWire tests the MsgDSProofWire wire encode and decode for
// various protocol versions.
func TestDSProofWire(t *testing.T) {
	msgSendCmpct := NewMsgDSProof(OutPoint{Index: 1}, Spender{PushData: make([][]byte, 0)}, Spender{PushData: make([][]byte, 0)})
	msgSendCmpctEncoded := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00}

	tests := []struct {
		in   *MsgDSProof     // Message to encode
		out  *MsgDSProof     // Expected decoded message
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

		// Protocol version DoubleSpendProofVersion+1
		{
			msgSendCmpct,
			msgSendCmpct,
			msgSendCmpctEncoded,
			DoubleSpendProofVersion + 1,
			BaseEncoding,
		},

		// Protocol version DoubleSpendProofVersion
		{
			msgSendCmpct,
			msgSendCmpct,
			msgSendCmpctEncoded,
			DoubleSpendProofVersion,
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
		var msg MsgDSProof
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

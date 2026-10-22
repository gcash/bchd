// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
)

// scriptPair is a signature script and the public key script it spends.
type scriptPair struct {
	sigScript, pkScript []byte
}

// analysisCorpus collects scripts and script pairs, without duplicates.
type analysisCorpus struct {
	scripts [][]byte
	pairs   []scriptPair
	seen    map[string]struct{}

	// numBase is the number of scripts that do not come from the VMB
	// vectors.  They come first.
	numBase int
}

// add adds scripts to the corpus.
func (c *analysisCorpus) add(scripts ...[]byte) {
	for _, s := range scripts {
		if _, ok := c.seen[string(s)]; ok {
			continue
		}
		c.seen[string(s)] = struct{}{}
		c.scripts = append(c.scripts, s)
	}
}

// addPair adds a script pair and both of its scripts to the corpus.
func (c *analysisCorpus) addPair(sigScript, pkScript []byte) {
	c.add(sigScript, pkScript)
	c.pairs = append(c.pairs, scriptPair{sigScript, pkScript})
}

// addVectors adds the scripts of the script_tests, tx_valid and tx_invalid
// test vectors.
func (c *analysisCorpus) addVectors(t testing.TB) {
	var scriptTests [][]interface{}
	for _, name := range []string{"script_tests.json", "script_tests_bchd.json"} {
		file, err := os.ReadFile("data/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var tests [][]interface{}
		if err := json.Unmarshal(file, &tests); err != nil {
			t.Fatal(err)
		}
		scriptTests = append(scriptTests, tests...)
	}
	for _, test := range scriptTests {
		// Rows are [[amount]] sig, pk, flags, result[, comment].
		if len(test) > 0 {
			if _, ok := test[0].([]interface{}); ok {
				test = test[1:]
			}
		}
		if len(test) < 2 {
			continue
		}
		sigStr, ok1 := test[0].(string)
		pkStr, ok2 := test[1].(string)
		if !ok1 || !ok2 {
			continue
		}
		sigScript, err1 := parseShortForm(sigStr)
		pkScript, err2 := parseShortForm(pkStr)
		if err1 != nil || err2 != nil {
			continue
		}
		c.addPair(sigScript, pkScript)
	}

	for _, name := range []string{"tx_valid.json", "tx_invalid.json"} {
		file, err := os.ReadFile("data/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var tests [][]interface{}
		if err := json.Unmarshal(file, &tests); err != nil {
			t.Fatal(err)
		}
		for _, test := range tests {
			inputs, ok := test[0].([]interface{})
			if !ok || len(test) < 2 {
				continue
			}
			txHex, ok := test[1].(string)
			if !ok {
				continue
			}
			txBytes, err := hex.DecodeString(txHex)
			if err != nil {
				continue
			}
			var tx wire.MsgTx
			if err := tx.Deserialize(bytes.NewReader(txBytes)); err != nil {
				continue
			}
			prevScripts := make(map[wire.OutPoint][]byte)
			for _, input := range inputs {
				in, ok := input.([]interface{})
				if !ok || len(in) < 3 {
					continue
				}
				hashStr, ok1 := in[0].(string)
				idx, ok2 := in[1].(float64)
				pkStr, ok3 := in[2].(string)
				if !ok1 || !ok2 || !ok3 {
					continue
				}
				pkScript, err := parseShortForm(pkStr)
				if err != nil {
					continue
				}
				var op wire.OutPoint
				if err := decodeHashStr(&op, hashStr); err != nil {
					continue
				}
				op.Index = testVecF64ToUint32(idx)
				prevScripts[op] = pkScript
			}
			for _, txIn := range tx.TxIn {
				if pkScript, ok := prevScripts[txIn.PreviousOutPoint]; ok {
					c.addPair(txIn.SignatureScript, pkScript)
				} else {
					c.add(txIn.SignatureScript)
				}
			}
			for _, txOut := range tx.TxOut {
				c.add(txOut.PkScript)
			}
		}
	}
}

// decodeHashStr sets the hash of op from its byte-reversed hex form.
func decodeHashStr(op *wire.OutPoint, s string) error {
	b, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return op.Hash.SetBytes(b)
}

// addVMBVectors adds the scripts of every VMB test vector: signature
// scripts, the redeem scripts they push last, the spent scripts with and
// without their token prefix, and the output scripts.
func (c *analysisCorpus) addVMBVectors(t testing.TB) {
	files, err := filepath.Glob("data/vmb_tests/*/*vmb_tests.json*")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no VMB test vectors found")
	}
	for _, name := range files {
		var file []byte
		if strings.HasSuffix(name, ".gz") {
			file, err = ReadGzFile(name)
		} else {
			file, err = os.ReadFile(name)
		}
		if err != nil {
			t.Fatal(err)
		}
		var tests [][]interface{}
		if err := json.Unmarshal(file, &tests); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, test := range tests {
			if len(test) < 6 {
				continue
			}
			txHex, ok1 := test[4].(string)
			utxoHex, ok2 := test[5].(string)
			if !ok1 || !ok2 {
				continue
			}
			txBytes, err1 := hex.DecodeString(txHex)
			utxoBytes, err2 := hex.DecodeString(utxoHex)
			if err1 != nil || err2 != nil {
				continue
			}
			var tx wire.MsgTx
			if err := tx.BchDecode(bytes.NewReader(txBytes), 0, 0); err != nil {
				continue
			}
			_, rawScripts := readRawUtxoScripts(utxoBytes)
			for i, txIn := range tx.TxIn {
				sigScript := txIn.SignatureScript
				c.add(sigScript)
				if pops, err := parseScript(sigScript); err == nil && len(pops) > 0 {
					c.add(pops[len(pops)-1].data)
				}
				if i >= len(rawScripts) {
					continue
				}
				raw := rawScripts[i]
				c.add(raw)
				var td wire.TokenData
				pkScript, err := td.SeparateTokenDataFromPKScriptIfExists(raw, 0)
				if err != nil {
					continue
				}
				c.addPair(sigScript, pkScript)
			}
			for _, txOut := range tx.TxOut {
				c.add(txOut.PkScript)
			}
		}
	}
}

// readRawUtxoScripts returns the values and serialized scripts, token
// prefix included, of a VMB utxo list.
func readRawUtxoScripts(b []byte) ([]int64, [][]byte) {
	r := bytes.NewReader(b)
	count, err := wire.ReadVarInt(r, 0)
	if err != nil || count > uint64(len(b)) {
		return nil, nil
	}
	values := make([]int64, 0, count)
	scripts := make([][]byte, 0, count)
	for range count {
		var value [8]byte
		if _, err := io.ReadFull(r, value[:]); err != nil {
			return values, scripts
		}
		n, err := wire.ReadVarInt(r, 0)
		if err != nil || n > uint64(r.Len()) {
			return values, scripts
		}
		script := make([]byte, n)
		if _, err := io.ReadFull(r, script); err != nil {
			return values, scripts
		}
		values = append(values, int64(binary.LittleEndian.Uint64(value[:])))
		scripts = append(scripts, script)
	}
	return values, scripts
}

// pushData returns the script that pushes data with the given push opcode,
// which need not be the minimal one.
func pushData(op byte, data []byte) []byte {
	switch op {
	case OP_PUSHDATA1:
		return append([]byte{op, byte(len(data))}, data...)
	case OP_PUSHDATA2:
		var l [2]byte
		binary.LittleEndian.PutUint16(l[:], uint16(len(data)))
		return append(append([]byte{op}, l[:]...), data...)
	case OP_PUSHDATA4:
		var l [4]byte
		binary.LittleEndian.PutUint32(l[:], uint32(len(data)))
		return append(append([]byte{op}, l[:]...), data...)
	}
	return append([]byte{byte(len(data))}, data...)
}

// catScripts concatenates scripts.
func catScripts(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

// analysisTemplates returns the standard script templates, near misses, and
// their forms with non-minimal pushes and token prefixes.
func analysisTemplates() [][]byte {
	pk33 := append([]byte{0x02}, bytes.Repeat([]byte{0x11}, 32)...)
	pk33b := append([]byte{0x03}, bytes.Repeat([]byte{0x79}, 32)...)
	pk65 := append([]byte{0x04}, bytes.Repeat([]byte{0x22}, 64)...)
	h20 := bytes.Repeat([]byte{0x33}, 20)
	h32 := bytes.Repeat([]byte{0x44}, 32)
	sig := append(bytes.Repeat([]byte{0x30}, 71), 0x41)
	pushOps := []byte{0, OP_PUSHDATA1, OP_PUSHDATA2, OP_PUSHDATA4}
	push := func(d []byte) []byte { return pushData(0, d) }

	// A real pubkey, so ExtractPkScriptAddrs and ComputePkScript see one.
	realPK, _ := hex.DecodeString("0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")

	var templates [][]byte
	add := func(s ...[]byte) { templates = append(templates, s...) }
	add(nil, []byte{})
	for _, op := range pushOps {
		for _, pk := range [][]byte{pk33, pk65, realPK, pk33[:32], make([]byte, 33)} {
			add(catScripts(pushData(op, pk), []byte{OP_CHECKSIG}))
		}
		add(catScripts([]byte{OP_DUP, OP_HASH160}, pushData(op, h20),
			[]byte{OP_EQUALVERIFY, OP_CHECKSIG}))
		add(catScripts([]byte{OP_HASH160}, pushData(op, h20), []byte{OP_EQUAL}))
		add(catScripts([]byte{OP_HASH256}, pushData(op, h32), []byte{OP_EQUAL}))
		add(catScripts([]byte{OP_1}, pushData(op, pk33), []byte{OP_1, OP_CHECKMULTISIG}))
		add(catScripts([]byte{OP_2}, push(pk33), pushData(op, pk65), push(realPK),
			[]byte{OP_3, OP_CHECKMULTISIG}))
		add(catScripts([]byte{OP_RETURN}, pushData(op, make([]byte, 80))))
		add(catScripts([]byte{OP_RETURN}, pushData(op, make([]byte, 200))))
	}

	// P2SH and P2SH32 with the other hash length, opcode or length byte.
	add(catScripts([]byte{OP_HASH160}, push(h32), []byte{OP_EQUAL}),
		catScripts([]byte{OP_HASH256}, push(h20), []byte{OP_EQUAL}),
		catScripts([]byte{OP_HASH160}, push(h20), []byte{OP_EQUALVERIFY}),
		catScripts([]byte{OP_SHA256}, push(h32), []byte{OP_EQUAL}),
		catScripts([]byte{OP_HASH160, OP_DATA_21}, h20, []byte{OP_EQUAL}),
		catScripts([]byte{OP_HASH256, OP_DATA_31}, h32, []byte{OP_EQUAL}),
		catScripts([]byte{OP_DUP, OP_HASH160}, push(h32), []byte{OP_EQUALVERIFY, OP_CHECKSIG}))

	// Multisig edge cases: OP_0 counts, m > n, n = 0, wrong n, too many keys,
	// keys of other lengths and small ints in place of keys.
	add(catScripts([]byte{OP_0}, push(pk33), []byte{OP_1, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_3}, push(pk33), push(pk33b), []byte{OP_2, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_16}, push(pk33), []byte{OP_1, OP_CHECKMULTISIG}),
		[]byte{OP_1, OP_0, OP_CHECKMULTISIG},
		[]byte{OP_0, OP_0, OP_CHECKMULTISIG},
		catScripts([]byte{OP_0}, []byte{OP_0, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1}, push(pk33), push(pk33), []byte{OP_1, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1}, push(pk33), []byte{OP_2, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1}, push(pk33), []byte{OP_1, OP_CHECKMULTISIGVERIFY}),
		catScripts([]byte{OP_1}, push(pk33), []byte{OP_1NEGATE, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1NEGATE}, push(pk33), []byte{OP_1, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1}, push(pk33[:32]), []byte{OP_1, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1}, push(h20), []byte{OP_1, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1}, []byte{OP_1}, []byte{OP_1, OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1}, push(pk33), []byte{OP_1, OP_CHECKMULTISIG, OP_NOP}),
		catScripts([]byte{OP_1}, push(pk33), []byte{OP_CHECKMULTISIG}),
		catScripts([]byte{OP_1}, push(pk33), []byte{OP_1, OP_CHECKMULTISIG, OP_DATA_1}))
	keys := []byte{OP_16}
	for range 16 {
		keys = append(keys, push(pk33)...)
	}
	add(catScripts(keys, []byte{OP_16, OP_CHECKMULTISIG}))
	add(catScripts(keys, push(pk33), []byte{OP_16, OP_CHECKMULTISIG}))
	add(catScripts(keys, push(pk33), []byte{OP_DATA_1, 17, OP_CHECKMULTISIG}))

	// Null data around the size limit, with every push kind and the
	// opcodes up to OP_16 that are not small ints.
	for n := MaxDataCarrierSize - 6; n <= MaxDataCarrierSize+2; n++ {
		add(catScripts([]byte{OP_RETURN, OP_PUSHDATA1, byte(n - 3)}, make([]byte, n-3)))
		add(catScripts([]byte{OP_RETURN, OP_PUSHDATA2, byte(n - 4), 0}, make([]byte, n-4)))
		add(catScripts([]byte{OP_RETURN, OP_PUSHDATA4, byte(n - 6), 0, 0, 0}, make([]byte, n-6)))
		add(catScripts([]byte{OP_RETURN}, bytes.Repeat([]byte{OP_1NEGATE}, n-1)))
	}
	add([]byte{OP_RETURN},
		[]byte{OP_RETURN, OP_RETURN},
		[]byte{OP_RETURN, OP_0, OP_1NEGATE, OP_RESERVED, OP_1, OP_16},
		[]byte{OP_RETURN, OP_NOP},
		[]byte{OP_RETURN, OP_PUSHDATA1},
		[]byte{OP_RETURN, OP_DATA_2, 1},
		[]byte{OP_0, OP_RETURN})

	// Signature scripts: P2PKH spends, P2SH spends and push-only edge cases.
	redeem := catScripts([]byte{OP_2}, push(pk33), push(realPK), []byte{OP_2, OP_CHECKMULTISIG})
	add(catScripts(push(sig), push(realPK)),
		catScripts(push(sig[:71]), push(realPK)),
		catScripts(push(sig), push(pk65)),
		catScripts([]byte{OP_0}, push(sig), push(sig), pushData(OP_PUSHDATA1, redeem)),
		catScripts(push(sig), pushData(OP_PUSHDATA2, redeem)),
		catScripts(push(sig), []byte{OP_NOP}, push(redeem)),
		catScripts(push(sig), []byte{OP_RESERVED}),
		catScripts(push(sig), []byte{OP_1NEGATE, OP_16, OP_0}),
		catScripts(push(sig), []byte{OP_DATA_5, 1}),
		[]byte{OP_0},
		[]byte{OP_PUSHDATA1, 0},
		[]byte{OP_PUSHDATA4, 0, 0, 0, 0})

	// Atomic swap contracts.
	swap := catScripts([]byte{OP_IF, OP_SIZE}, push([]byte{32}),
		[]byte{OP_EQUALVERIFY, OP_SHA256}, push(h32),
		[]byte{OP_EQUALVERIFY, OP_DUP, OP_HASH160}, push(h20),
		[]byte{OP_ELSE}, push([]byte{0x80, 0x96, 0x98, 0x00}),
		[]byte{OP_CHECKLOCKTIMEVERIFY, OP_DROP, OP_DUP, OP_HASH160}, push(h20),
		[]byte{OP_ENDIF, OP_EQUALVERIFY, OP_CHECKSIG})
	add(swap, catScripts(swap, []byte{OP_NOP}), catScripts(swap, []byte{OP_DATA_1}))

	// Token prefixes before the standard templates: an NFT, an amount
	// and a truncated prefix.
	category := bytes.Repeat([]byte{0x55}, 32)
	nft := catScripts([]byte{SPECIAL_TOKEN_PREFIX}, category, []byte{0x20})
	ft := catScripts([]byte{SPECIAL_TOKEN_PREFIX}, category, []byte{0x10, 0x64})
	for _, s := range templates[2:20] {
		add(catScripts(nft, s), catScripts(ft, s), catScripts([]byte{SPECIAL_TOKEN_PREFIX}, s))
	}
	return templates
}

// analysisInteresting holds the bytes the mutations use: small ints, push
// opcodes, pubkey prefixes and the opcodes the templates match on.
var analysisInteresting = []byte{OP_0, OP_1, OP_2, OP_3, OP_16, OP_1NEGATE,
	OP_RESERVED, OP_DATA_1, OP_DATA_20, OP_DATA_32, OP_DATA_33, OP_DATA_65,
	OP_PUSHDATA1, OP_PUSHDATA2, OP_PUSHDATA4, OP_DUP, OP_HASH160, OP_HASH256,
	OP_EQUAL, OP_EQUALVERIFY, OP_CHECKSIG, OP_CHECKMULTISIG, OP_RETURN, OP_NOP,
	SPECIAL_TOKEN_PREFIX, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0xff}

// mutationOffsets returns the offsets of the opcodes in script, and of the
// two bytes after each, which hold push lengths and pubkey prefixes.
func mutationOffsets(script []byte) []int {
	var offsets []int
	seen := make(map[int]bool)
	tok := MakeScriptTokenizer(script)
	for i := 0; i < len(script); i = tok.ByteIndex() {
		for j := i; j < i+3 && j < len(script); j++ {
			if !seen[j] {
				seen[j] = true
				offsets = append(offsets, j)
			}
		}
		if !tok.Next() {
			break
		}
	}
	if len(script) > 0 && !seen[len(script)-1] {
		offsets = append(offsets, len(script)-1)
	}
	return offsets
}

// addSynthetic adds the templates, single byte mutations, truncations and
// extensions of them, and random scripts.
func (c *analysisCorpus) addSynthetic() {
	templates := analysisTemplates()
	c.add(templates...)
	for _, s := range templates {
		for _, i := range mutationOffsets(s) {
			for _, b := range analysisInteresting {
				m := bytes.Clone(s)
				m[i] = b
				c.add(m)
			}
		}
		for i := range s {
			c.add(bytes.Clone(s[:i]), catScripts(s[:i], s[i+1:]))
		}
		for _, b := range analysisInteresting {
			c.add(catScripts(s, []byte{b}), catScripts([]byte{b}, s))
		}
	}

	rng := rand.New(rand.NewSource(2))
	for range 20000 {
		c.add(randomScript(rng))
		var s []byte
		for range rng.Intn(6) {
			if rng.Intn(2) == 0 {
				s = append(s, templates[rng.Intn(len(templates))]...)
			} else {
				s = append(s, analysisInteresting[rng.Intn(len(analysisInteresting))])
			}
		}
		c.add(s)
	}
}

// newAnalysisCorpus returns the corpus the differential tests run on.  The
// VMB vectors are left out in short mode.
func newAnalysisCorpus(t testing.TB, vmb bool) *analysisCorpus {
	c := &analysisCorpus{seen: make(map[string]struct{})}
	c.addVectors(t)
	c.addSynthetic()
	c.numBase = len(c.scripts)
	if vmb {
		c.addVMBVectors(t)
	}
	return c
}

// panicValue runs f and returns what it panicked with, if anything.
func panicValue(f func()) (v interface{}) {
	defer func() { v = recover() }()
	f()
	return nil
}

// analysisEngineFlags are the flag sets the NewEngine checks are compared
// under.
var analysisEngineFlags = []ScriptFlags{
	0,
	ScriptBip16,
	ScriptVerifySigPushOnly,
	ScriptVerifyCleanStack,
	ScriptBip16 | ScriptVerifyCleanStack | ScriptVerifySigPushOnly,
	StandardVerifyFlags | ScriptAllowCashTokens | ScriptAllowMay2025 |
		ScriptAllowMay2026,
}

// checkScriptAnalysis ensures the tokenizer-based analysis functions give
// the same results as the parsedOpcode-based reference on one script.
func checkScriptAnalysis(t testing.TB, script []byte) {
	t.Helper()

	same := func(name string, got, want interface{}) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s(%x):\n got %#v\nwant %#v", name, script, got, want)
		}
	}
	type result []interface{}

	same("GetScriptClass", GetScriptClass(script), refGetScriptClass(script))
	same("IsPushOnlyScript", IsPushOnlyScript(script), refIsPushOnlyScript(script))
	same("IsPayToScriptHash", IsPayToScriptHash(script), refIsPayToScriptHash(script))
	same("IsPayToScriptHash32", IsPayToScriptHash32(script), refIsPayToScriptHash32(script))

	data, err := PushedData(script)
	refData, refErr := refPushedData(script)
	same("PushedData", result{data, err}, result{refData, refErr})

	data, err = ExtractDataElements(script)
	refData, refErr = refExtractDataElements(script)
	same("ExtractDataElements", result{data, err}, result{refData, refErr})

	var n, m, rn, rm int
	p := panicValue(func() { n, m, err = CalcMultiSigStats(script) })
	rp := panicValue(func() { rn, rm, refErr = refCalcMultiSigStats(script) })
	same("CalcMultiSigStats", result{n, m, err, p != nil},
		result{rn, rm, refErr, rp != nil})

	for _, params := range []*chaincfg.Params{&chaincfg.MainNetParams,
		&chaincfg.TestNet3Params} {

		class, addrs, reqSigs, err := ExtractPkScriptAddrs(script, params)
		rClass, rAddrs, rReqSigs, rErr := refExtractPkScriptAddrs(script, params)
		same("ExtractPkScriptAddrs", result{class, addrs, reqSigs, err},
			result{rClass, rAddrs, rReqSigs, rErr})
	}

	swap, err := ExtractAtomicSwapDataPushes(0, script)
	rSwap, rErr := refExtractAtomicSwapDataPushes(0, script)
	same("ExtractAtomicSwapDataPushes", result{swap, err}, result{rSwap, rErr})

	pk, err := ComputePkScript(script)
	rPk, rErr := refComputePkScript(script)
	same("ComputePkScript", result{pk, err}, result{rPk, rErr})
}

// checkPairAnalysis ensures the tokenizer-based functions that look at a
// signature script and the script it spends, including the checks at the
// start of NewEngine, match the reference.
func checkPairAnalysis(t testing.TB, sigScript, pkScript []byte) {
	t.Helper()

	same := func(name string, got, want interface{}) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s(%x, %x):\n got %#v\nwant %#v", name, sigScript,
				pkScript, got, want)
		}
	}
	type result []interface{}

	for _, flags := range []ScriptFlags{0, ScriptBip16} {
		var info, rInfo *ScriptInfo
		var err, rErr error
		p := panicValue(func() { info, err = CalcScriptInfo(sigScript, pkScript, flags) })
		rp := panicValue(func() { rInfo, rErr = refCalcScriptInfo(sigScript, pkScript, flags) })
		same("CalcScriptInfo", result{info, err, p != nil},
			result{rInfo, rErr, rp != nil})
	}

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{SignatureScript: sigScript})
	tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{OP_TRUE}})
	for _, flags := range analysisEngineFlags {
		scriptIdx, bip16 := 0, false
		vm, err := NewEngine(pkScript, tx, 0, flags, nil, nil, nil, 0)
		if err == nil {
			scriptIdx, bip16 = vm.scriptIdx, vm.bip16
		}
		rScriptIdx, rBip16, rErr := refNewEngineChecks(pkScript, tx, 0, flags)
		same("NewEngine", result{scriptIdx, bip16, err},
			result{rScriptIdx, rBip16, rErr})
	}
}

// TestAnalysisMatchesReference ensures the tokenizer-based script analysis
// functions give the same results as the parsedOpcode-based ones they
// replaced on the test vectors, standard templates, mutations of them and
// random scripts.
func TestAnalysisMatchesReference(t *testing.T) {
	t.Parallel()

	c := newAnalysisCorpus(t, !testing.Short())
	for _, script := range c.scripts {
		checkScriptAnalysis(t, script)
	}
	for _, pair := range c.pairs {
		checkPairAnalysis(t, pair.sigScript, pair.pkScript)
	}

	// Pair the scripts that are not from the VMB vectors with P2SH and
	// P2SH32 scripts, and with each other.
	p2sh := mustParseShortForm("HASH160 DATA_20 0x" + strings.Repeat("00", 20) + " EQUAL")
	p2sh32 := mustParseShortForm("HASH256 DATA_32 0x" + strings.Repeat("00", 32) + " EQUAL")
	base := c.scripts[:c.numBase]
	for i, script := range base {
		var pkScript []byte
		switch i % 3 {
		case 0:
			pkScript = p2sh
		case 1:
			pkScript = p2sh32
		default:
			pkScript = base[(i*7919)%len(base)]
		}
		checkPairAnalysis(t, script, pkScript)
	}
	t.Logf("compared %d scripts and %d pairs", len(c.scripts),
		len(c.pairs)+len(base))
}

// FuzzAnalysisMatchesReference ensures the tokenizer-based script analysis
// functions give the same results as the parsedOpcode-based ones on any
// script and script pair.
func FuzzAnalysisMatchesReference(f *testing.F) {
	c := newAnalysisCorpus(f, false)
	rng := rand.New(rand.NewSource(3))
	for _, pair := range c.pairs {
		f.Add(pair.sigScript, pair.pkScript)
	}
	for range 2000 {
		f.Add(c.scripts[rng.Intn(len(c.scripts))], c.scripts[rng.Intn(len(c.scripts))])
	}
	f.Fuzz(func(t *testing.T, sigScript, pkScript []byte) {
		checkScriptAnalysis(t, sigScript)
		checkScriptAnalysis(t, pkScript)
		checkPairAnalysis(t, sigScript, pkScript)
	})
}

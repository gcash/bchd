package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/golang/glog"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

const bchdTestNode = "bchd.greyh.at:8335"
const logRequestJSON = true // log JSON of request and responses (to glog)

const dustLimit = 546

var httpClient *HTTPClient

func TestMain(m *testing.M) {
	var err error
	flag.Parse()
	defer glog.Flush()

	// use a remote BCHD full node as default (otherwise we would have to sync the chain fully on localhost)
	if !isFlagPresent("bchd-grpc-url") {
		testFullNodeEndpoint := bchdTestNode
		grpcServerEndpoint = &testFullNodeEndpoint
	}

	log.Println("Starting local test HTTP server...")

	// Create the app context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testProxyAddr := ":8282"
	proxyAddr = &testProxyAddr

	proxy := &GrpcProxy{
		ctx:    ctx,
		server: nil,
	}
	go startLocalTestServer(proxy)
	time.Sleep(1 * time.Second) // wait for HTTP server goroutine

	// connect the HTTP test client to address:port
	httpAddr := *proxyAddr
	urlObj, err := url.Parse("http://" + httpAddr)
	if err != nil {
		log.Printf("Error parsing proxy backend address %s: %+v", httpAddr, err)
		os.Exit(1)
	}
	if len(urlObj.Host) != 0 && urlObj.Host[0:1] == ":" {
		httpAddr = "localhost" + httpAddr // only port specified -> use localhost
	}
	httpClient, err = newHTTPClient(fmt.Sprintf("http://%s/v1/", httpAddr), logRequestJSON)
	if err != nil {
		log.Printf("Error creating HTTP client: %+v", err)
		os.Exit(1)
	}

	log.Println("Test environment is ready. Starting tests")
	exitVal := m.Run() // run tests

	// stop local server & cleanup
	proxy.Shutdown()

	os.Exit(exitVal)
}

func TestGetBlockchainInfo(t *testing.T) {
	method := "GetBlockchainInfo"
	minBlockHeight := 668619 // to ensure our node returns expected data on other API calls
	//res, err := httpClient.RequestMap(method, D{})
	res, err := httpClient.RequestRaw(method, D{})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	var resMap D
	err = json.Unmarshal(res, &resMap)
	if err != nil {
		t.Fatalf("error unmarshalling %s API data %+v", method, err)
	}
	if int(resMap["best_height"].(float64)) < minBlockHeight {
		// strange that best_height is float64 when marshalling to interface{}. better marshall to proto structs to get expected types
		// we marshall to pb structs in other tests. But keep 1 check with map to ensure our JSON uses snake case property names
		t.Fatalf("Your node is not fully synced. Some requests will likely return incorrect data. Best height %.0f, required height %d", resMap["best_height"].(float64), minBlockHeight)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	t.Logf("Successfully passed %s test - best block height %.0f", method, resMap["best_height"].(float64))
}

func TestResponseHeaders(t *testing.T) {
	method := "GetBlockchainInfo"
	res, err := httpClient.Request(method, D{})
	if err != nil {
		t.Fatalf("%s requst failed: %+v", method, err)
	}
	defer res.Body.Close()

	if res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("%s is missing expected Cache-Control header", method)
	} else if res.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("%s is missing expected Access-Control-Allow-Origin header", method)
	} else {
		t.Logf("%s contains expected HTTP headers", method)
	}
}

func TestOptionsRequest(t *testing.T) {
	method := "GetTransaction"
	urlStr := httpClient.GetMethodURL(method)
	urlObj, _ := url.Parse(urlStr)
	resp, err := httpClient.Client.Do(&http.Request{
		Method: "OPTIONS",
		URL:    urlObj,
	})
	if err != nil {
		t.Fatalf("%s requst failed: %+v", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("%s OPTIONS request got invalid status code %d", method, resp.StatusCode)
	}
	t.Logf("%s OPTIONS request got expected status code %d", method, resp.StatusCode)
}

func TestRequestInvalid(t *testing.T) {
	// ensure BCHD doesn't crash or return unexpected data
	method := "GetTransaction"
	res, err := httpClient.RequestMap(method, D{
		"hash":                   "some-invalid-hash",
		"include_token_metadata": true,
	})
	if err == nil {
		t.Fatalf("%s was expected to fail with invalid params. Response: %+v", method, res)
	}
	t.Logf("Successfully tested %s - received %+v", method, err)
}

func TestGetTransaction(t *testing.T) {
	method := "GetTransaction"
	txHash, _ := hex.DecodeString("15388bfd9998429b2955700da25d22178658cee8a9037423793a94efc047fbed")
	txHashBase64 := base64.StdEncoding.EncodeToString(reverseBytes(txHash))
	tokenID := "7278363093d3b899e0e1286ff681bf50d7ddc3c2a68565df743d0efc54c0e7fd"
	tokenIDBytes, _ := hex.DecodeString(tokenID)

	res, err := httpClient.RequestRaw(method, D{
		"hash":                   txHashBase64,
		"include_token_metadata": true,
	})
	if err != nil {
		t.Fatalf("%s failed. Response: %+v", method, err)
	}

	// ignore missing address on first output which is op_return in this case
	ignores := []string{"transaction.outputs.0.address: String length must be greater than or equal to 42"}

	if err := validateJSONSchema(method, res, ignores); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var tx pb.GetTransactionResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &tx)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if len(tx.Transaction.Inputs) != 2 {
		t.Fatalf("Expected 2 inputs on %s, got %d", method, len(tx.Transaction.Inputs))
	} else if len(tx.Transaction.Outputs) != 4 {
		t.Fatalf("Expected 4 outputs on %s, got %d", method, len(tx.Transaction.Outputs))
	} else if tx.TokenMetadata == nil {
		t.Fatalf("Missing token metadata on %s", method)
	} else if !bytes.Equal(tokenIDBytes, tx.TokenMetadata.TokenId) {
		t.Fatalf("Wrong token ID on %s: %x", method, tx.TokenMetadata.TokenId)
	}

	t.Logf("Successfully tested %s", method)
}

func TestGetAddressUnspentOutputs(t *testing.T) {
	method := "GetAddressUnspentOutputs"
	address := "simpleledger:qq9djgun97em5arkvqa9le0wdstlv8lmdvgu0xc2c2"
	res, err := httpClient.RequestRaw(method, D{
		"address":                address,
		"include_mempool":        true,
		"include_token_metadata": true,
	})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var outputs pb.GetAddressUnspentOutputsResponse
	// we use the better runtime.JSONPb marshaller and not runtime.JSONBuiltin so the official golang json can't marshall everything back properly
	// specifically: outputs.value is a string in our JSON (int64 too large for JS number)
	//err = json.Unmarshal(res, &outputs)
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &outputs)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if len(outputs.Outputs) < 1 {
		t.Fatalf("%s has no outputs for address %s", method, address)
	}

	firstOutput := outputs.Outputs[0]
	var minHeight int32 = 668417
	if firstOutput.BlockHeight < minHeight {
		t.Errorf("%s first output BlockHeight must be >= %d, received %d", method, minHeight, firstOutput.BlockHeight)
	} else if firstOutput.Value < dustLimit {
		t.Errorf("%s first output should have value >= dust limit, received %d", method, firstOutput.Value)
	} else if outputs.TokenMetadata == nil || len(outputs.TokenMetadata) == 0 {
		t.Errorf("expected token metadata in %s", method)
	} else {
		t.Logf("Successfully passed %s test. Got %d ouputs", method, len(outputs.Outputs))
	}
}

func TestGetCashAddressUnspentOutputs(t *testing.T) {
	method := "GetAddressUnspentOutputs"
	address := "bitcoincash:qz7j7805n9yjdccpz00gq7d70k3h3nef9yj0pwpelz"
	res, err := httpClient.RequestRaw(method, D{
		"address":                address,
		"include_mempool":        false,
		"include_token_metadata": false,
	})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var outputs pb.GetAddressUnspentOutputsResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &outputs)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if len(outputs.Outputs) < 1 {
		t.Fatalf("%s has no outputs for address %s", method, address)
	}

	t.Logf("Successfully passed %s test. Got %d ouputs", method, len(outputs.Outputs))
}

func TestGetAddressUnspentOutputsEmpty(t *testing.T) {
	method := "GetAddressUnspentOutputs"
	address := "simpleledger:qpfdgdftjj43f9fhzm2k4ysrcuwlae2l3vd4pvmhy7"
	res, err := httpClient.RequestRaw(method, D{
		"address":                address,
		"include_mempool":        true,
		"include_token_metadata": true,
	})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var outputs pb.GetAddressUnspentOutputsResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &outputs)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if len(outputs.Outputs) != 0 {
		t.Fatalf("%s is expected to have no outputs for address %s", method, address)
	} else if len(outputs.TokenMetadata) != 0 {
		t.Fatalf("%s is expected to have no token metadata for address %s", method, address)
	}
	t.Logf("Successfully passed %s test. Got %s", method, outputs.String())
}

func TestGetTokenBalance(t *testing.T) {
	method := "GetAddressUnspentOutputs"
	tokenID := "0be40e351ea9249b536ec3d1acd4e082e860ca02ec262777259ffe870d3b5cc3"
	tokenIDBytes, _ := hex.DecodeString(tokenID)
	address := "simpleledger:qz7j7805n9yjdccpz00gq7d70k3h3nef9y75245epu"
	res, err := httpClient.RequestRaw(method, D{
		"address":                address,
		"include_mempool":        true,
		"include_token_metadata": true,
	})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var outputs pb.GetAddressUnspentOutputsResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &outputs)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if len(outputs.Outputs) == 0 {
		t.Fatalf("%s is expected to have outputs for address %s", method, address)
	} else if len(outputs.TokenMetadata) == 0 {
		t.Fatalf("%s is expected to have token metadata for address %s", method, address)
	}

	// get token balance
	var balance uint64
	for _, out := range outputs.Outputs {
		if out.SlpToken == nil {
			continue
		}
		if !bytes.Equal(out.SlpToken.TokenId, tokenIDBytes) {
			continue
		}
		balance += out.SlpToken.Amount
	}
	if balance == 0 {
		t.Fatalf("Unable to get token balance for address %s of token %s", address, tokenID)
	}
	t.Logf("Successfully got token balance %d of address %s", balance, address)
}

func TestGetSlpTokenMetadata(t *testing.T) {
	method := "GetSlpTokenMetadata"
	tokenID := "7278363093d3b899e0e1286ff681bf50d7ddc3c2a68565df743d0efc54c0e7fd"
	tokenIDBase64, _ := hexToBase64(tokenID)
	res, err := httpClient.RequestRaw(method, D{
		"token_ids": []string{tokenIDBase64},
	})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var meta pb.GetSlpTokenMetadataResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &meta)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if len(meta.TokenMetadata) != 1 {
		t.Fatalf("%s expected to return exacty 1 token info, got %d", method, len(meta.TokenMetadata))
	}
	token := meta.TokenMetadata[0]
	if hex.EncodeToString(token.TokenId) != tokenID {
		t.Errorf("%s returned wrong token ID %s", method, hex.EncodeToString(token.TokenId))
	} else if token.TypeMetadata == nil {
		t.Errorf("%s missing token type metadata token ID %s", method, hex.EncodeToString(token.TokenId))
	}

	t.Logf("Successfully passed %s test for token %s", method, tokenID)
}

func TestCheckSlpTransaction(t *testing.T) {
	method := "CheckSlpTransaction"
	transaction := "0200000002d41b1a8d69c5bcdf3d8501b68eb27517d02b015a8c285e7857bccd66b32725bb010000006a47304402202f1e0939621d83758b41177bde092312b935c540afe6fec58e6ca7c057a2ff6e022039d0a11a533e6bede4e546a8d5b35e9d7013310972831c3bc70425f538ac0649412103de58c47c5263d8b049861d5a737fd52c1bb72c196f7ceff841c5189702b58830ffffffffd41b1a8d69c5bcdf3d8501b68eb27517d02b015a8c285e7857bccd66b32725bb030000006a47304402206f4a0a8db47f9f77e609ca3fc5dba6e6a9950829abc81c421a14dd80bf88e9930220664be287f234199138c1a2b739546949156d25e795b961670a34ceb503eedea0412103de58c47c5263d8b049861d5a737fd52c1bb72c196f7ceff841c5189702b58830ffffffff040000000000000000406a04534c500001010453454e44207278363093d3b899e0e1286ff681bf50d7ddc3c2a68565df743d0efc54c0e7fd080000001d8e36aefb0800000002deecde8022020000000000001976a9142940168352672c655e827c1748f5bf8601b6cbe588ac22020000000000001976a914f1d032a93fe7e71b79a25511dc7fc3bad1ea8d0c88ac6a1b0700000000001976a9142940168352672c655e827c1748f5bf8601b6cbe588ac00000000"

	transactionBase64, _ := hexToBase64(transaction)
	res, err := httpClient.RequestRaw(method, D{
		"transaction":        transactionBase64,
		"required_slp_burns": nil,
	})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var txRes pb.CheckSlpTransactionResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &txRes)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if txRes.IsValid == false {
		t.Fatalf("%s said TX is invalid: %s", method, txRes.String())
	}

	t.Logf("Successfully passed %s test", method)
}

func TestCheckSlpTransactionBurnAllowed(t *testing.T) {
	// input taken from SLP test: allows BURNED_INPUTS_OTHER_TOKEN, for ending a mint baton
	// https://github.com/simpleledgerinc/grpc-bchrpc-node/blob/master/test/client.spec.ts

	method := "CheckSlpTransaction"
	transactionBase64, _ := hexToBase64("0100000002cf47cdc10902e2423b3b4b4c1a750e0525f0dbdc80c9b1248d87d372822c297f020000006a47304402201abb1eb1bbee6f0a59cdeccafcf1e9f5f3609b2f1f8b06e363f99b909ddc0ccb02206cd852340c6f9749312b9a6d00059c19ec0016d6279f8442d88cc905c026e0cc412103326306e0c27c5cabcab4ff215dec7a1acb5e019b796fbd563cbc13e1176f6dacfeffffffe2dd8264c70e8da104b6113457c436096e84e4566c68dfc1e76dad8a54470117020000006b483045022100bd6a24c5c79b727515b642f6f0e7dd8ed78a326a88fafd8c61e20f7e35aa906602200b266953050d4d37eba14f34e3924277c882d08718a88377957687ef5e48437c412102e11b25ad09036672e09612cf14373bca526f976c1113d28e25de5fdedc50f054feffffff030000000000000000396a04534c50000101044d494e5420170147548aad6de7c1df686c56e4846e0936c4573411b604a18d0ec76482dde24c0008000000000000006422020000000000001976a91402ca8fafb2f521083fbb6f416b19878cc70e6de088ace31b0100000000001976a9148dffa75af65d0dc1769a8522ada9e3ec7806e4ba88ace14c0800")
	tokenIDBase64, _ := hexToBase64("170147548aad6de7c1df686c56e4846e0936c4573411b604a18d0ec76482dde2")
	outpointHash, _ := hex.DecodeString("170147548aad6de7c1df686c56e4846e0936c4573411b604a18d0ec76482dde2")
	outpointHashBase64 := base64.StdEncoding.EncodeToString(reverseBytes(outpointHash))

	res, err := httpClient.RequestRaw(method, D{
		"transaction": transactionBase64,
		"required_slp_burns": []D{{
			"outpoint": D{
				"hash":  outpointHashBase64,
				"index": 2,
			},
			"token_id":        tokenIDBase64,
			"token_type":      1,
			"mint_baton_vout": 2,
		}},
	})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var txRes pb.CheckSlpTransactionResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &txRes)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if txRes.IsValid == false {
		t.Fatalf("%s said TX is invalid: %s", method, txRes.String())
	}

	t.Logf("Successfully passed %s test", method)
}

func TestGetSlpParsedScript(t *testing.T) {
	method := "GetSlpParsedScript"
	slpScriptBase64 := "agRTTFAAAQEEU0VORCByeDYwk9O4meDhKG/2gb9Q193DwqaFZd90PQ78VMDn/QgAAABxis0TAAgAAAAC34V1AA=="
	tokenID, _ := base64.StdEncoding.DecodeString("cng2MJPTuJng4Shv9oG/UNfdw8KmhWXfdD0O/FTA5/0=")
	t.Logf("test %x", tokenID)

	res, err := httpClient.RequestRaw(method, D{
		"slp_opreturn_script": slpScriptBase64,
	})
	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var script pb.GetSlpParsedScriptResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &script)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if len(script.ParsingError) != 0 {
		t.Fatalf("%s returned script parsing error %s", method, script.ParsingError)
	} else if !bytes.Equal(tokenID, script.TokenId) {
		t.Fatalf("%s returned different token ID %x", method, script.TokenId)
	}
	sendData := script.GetV1Send()
	if sendData == nil || len(sendData.Amounts) != 2 {
		t.Fatalf("%s returned unexpected send data. Expected 2 v1Send amounts", method)
	}

	t.Logf("Successfully passed %s test", method)
}

func TestGetSlpTrustedValidation(t *testing.T) {
	method := "GetSlpTrustedValidation"
	transactionID, _ := hex.DecodeString("3ff425384539519e815507f7f6739d9c12a44af84ff895601606b85157e0fb19")
	transactionIDBase64 := base64.StdEncoding.EncodeToString(reverseBytes(transactionID))
	prevOutVout := 1

	res, err := httpClient.RequestRaw(method, D{
		"queries": []D{{
			"prev_out_hash":            transactionIDBase64,
			"prev_out_vout":            prevOutVout,
			"graphsearch_valid_hashes": nil,
		}},
		"include_graphsearch_count": true,
	})

	if err != nil {
		t.Fatalf("%s test failed: %+v", method, err)
	}

	if err := validateJSONSchema(method, res, nil); err != nil {
		t.Fatalf("Error validating %s JSON schema: %+v", method, err)
	}

	var validation pb.GetSlpTrustedValidationResponse
	marshaller := runtime.JSONPb{}
	err = marshaller.Unmarshal(res, &validation)
	if err != nil {
		t.Fatalf("Error unmarshalling %s response: %+v", method, err)
	} else if len(validation.Results) != 1 {
		t.Fatalf("%s expected to return 1 result. Got %d", method, len(validation.Results))
	}

	firstResult := validation.Results[0]
	if firstResult.SlpAction != pb.SlpAction_SLP_V1_SEND {
		t.Fatalf("%s expected to contain Action SLP_V1_SEND. Received %d", method, firstResult.SlpAction)
	} else if firstResult.TokenType != 0 {
		t.Fatalf("%s expected to contain token type 0. Received %d", method, firstResult.TokenType)
	}

	t.Logf("Successfully passed %s test", method)
}

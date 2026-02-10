package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"slices"
	"time"

	"github.com/golang/glog"
	"github.com/xeipuuv/gojsonschema"
)

// HTTPClient is the HTTP client we use for API calls.
type HTTPClient struct {
	Client         *http.Client
	LogRequestBody bool // log JSON request and response body. Remember to add --alsologtostderr=true for glog output
	apiURL         string
}

// D is a shortcut for map[string]interface{} for request and response data
type D map[string]interface{}

func newHTTPClient(apiURL string, logRequestBody bool) (*HTTPClient, error) {
	cookies, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &HTTPClient{
		Client: &http.Client{
			Jar:     cookies,
			Timeout: 10 * time.Second,
		},
		LogRequestBody: logRequestBody,
		apiURL:         apiURL,
	}
	return client, nil
}

// GetMethodURL provides formatting for method URL
func (c *HTTPClient) GetMethodURL(method string) string {
	return c.apiURL + method
}

// Request performs an API request returning the raw http.Response.
// Be sure to call resp.Body.Close() after reading.
func (c *HTTPClient) Request(method string, body D) (*http.Response, error) {
	jsonParams, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("unable to marshall request body %+v", err)
	}
	if c.LogRequestBody {
		glog.Infof("%s req: %s", method, jsonParams)
	}

	resp, err := c.Client.Post(c.GetMethodURL(method), "application/json", bytes.NewBuffer(jsonParams))
	if err != nil {
		return nil, fmt.Errorf("error calling REST API response %+v", err)
	}

	return resp, nil
}

// RequestRaw makes a REST API call to the given method with body as parameters.
func (c *HTTPClient) RequestRaw(method string, body D) ([]byte, error) {
	resp, err := c.Request(method, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Invalid status code on REST call %d - res: %s", resp.StatusCode, c.PrintRes(resp))
	}
	resData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading REST API data %+v", err)
	}
	if c.LogRequestBody {
		glog.Infof("%s res: %s", method, resData)
	}

	return resData, nil
}

// RequestMap makes a request and return the response in a map for easier parsing.
func (c *HTTPClient) RequestMap(method string, body D) (D, error) {
	resData, err := c.RequestRaw(method, body)
	if err != nil {
		return nil, err
	}

	var resJSON D
	err = json.Unmarshal(resData, &resJSON)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling REST API data %+v", err)
	}
	return resJSON, nil
}

// PrintRes gets the response code + message as string (for logging).
func (c *HTTPClient) PrintRes(resp *http.Response) string {
	resData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("error reading res %+v", err)
	}
	var resJSON D
	err = json.Unmarshal(resData, &resJSON)
	if err != nil {
		return fmt.Sprintf("invalid JSON res %+v", err)
	}
	return fmt.Sprintf("Status %d\n%v", resp.StatusCode, resJSON)
}

func startLocalTestServer(proxy *GrpcProxy) {
	// Create the app context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := proxy.serveHTTP(ctx); err != nil {
		glog.Fatalf("Error starting HTTP server: %+v", err)
	}
}

// Check if a known program argument (defined with flag package) is present.
func isFlagPresent(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func validateJSONSchema(apiMmethod string, res []byte, ignoreErrors []string) error {
	schemaLoader := gojsonschema.NewReferenceLoader(fmt.Sprintf("file://./test/schema/%s.json", apiMmethod))
	documentLoader := gojsonschema.NewBytesLoader(res)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}
	if result.Valid() {
		return nil
	}
	errMsg := ""
	for _, desc := range result.Errors() {
		if slices.Contains(ignoreErrors, desc.String()) {
			continue
		}
		errMsg += fmt.Sprintf("- %s\n", desc)
	}
	if errMsg == "" {
		return nil
	}
	errMsgFront := "The document is not valid. see errors :\n"
	return errors.New(errMsgFront + errMsg)
}

func base64ToHex(base64Str string) (string, error) {
	hexData, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hexData), nil

}

func hexToBase64(hexStr string) (string, error) {
	hexData, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(hexData), nil
}

func reverseBytes(input []byte) []byte {
	out := make([]byte, len(input))

	for i := 0; i < len(input); i++ {
		out[i] = input[len(input)-i-1]
	}
	return out
}

package encoding

import (
	"bytes"
	"io"
	"net/http"

	"github.com/fxamacker/cbor/v2"
)

// MarshalCBOR encodes data to CBOR format
func MarshalCBOR(v interface{}) ([]byte, error) {
	return cbor.Marshal(v)
}

// UnmarshalCBOR decodes CBOR data
func UnmarshalCBOR(data []byte, v interface{}) error {
	return cbor.Unmarshal(data, v)
}

// SendCBORRequest sends HTTP POST request with CBOR-encoded body
func SendCBORRequest(client *http.Client, url string, data interface{}, headers map[string]string) (*http.Response, error) {
	// Encode to CBOR
	body, err := MarshalCBOR(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// Set Content-Type to CBOR
	req.Header.Set("Content-Type", "application/cbor")

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return client.Do(req)
}

// ReadCBORResponse reads and decodes CBOR response
func ReadCBORResponse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return UnmarshalCBOR(body, v)
}

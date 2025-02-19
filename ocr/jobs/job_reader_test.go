package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// GetTOMLReader returns a bytes.Reader for the TOML string
func GetTOMLReader() *bytes.Reader {
	tomlString := `
type = "directrequest"
schemaVersion = 1
chainID = "ethereum"
evmChainID = 1
externalJobId = "8aa49abd-0437-4eca-8fd4-84e11119fb0b"
name = "minimal eth request"
contractAddress = "0x1234567890abcdef1234567890abcdef12345678"

[[observationSource]]
name = "fetch_data"
type = "http"
method = "GET"
url = "https://min-api.cryptocompare.com/data/price?fsym=ETH&tsyms=BTC,USD,EUR"

[[observationSource]]
name = "parse_json"
type = "json"
input = "fetch_data"
path = "USD"

[[observationSource]]
name = "multiply"
type = "math"
input = "parse_json"
factor = 1
`

	return bytes.NewReader([]byte(tomlString))
}

type PriceResponse struct {
	USD float64 `json:"USD"`
}

func getETHPriceInUSD() (float64, error) {
	// The API URL
	url := "https://min-api.cryptocompare.com/data/price?fsym=ETH&tsyms=BTC,USD,EUR"

	// Make the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Decode the JSON response
	var priceResp PriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return 0, err
	}

	// Return the ETH price in USD
	return priceResp.USD, nil
}

func TestDirectReqTomlConfigReader(t *testing.T) {
	factory := NewJobReaderFactory()
	config := JobReaderConfig{
		JobFormat: JobFormatTOML,
		JobType:   DirectReqJob,
	}
	job, err := factory.Read(GetTOMLReader(), config)
	assert.NoError(t, err)

	if job == nil {
		t.Errorf("Expected a reader, got nil")
	}

	assert.Equal(t, "8aa49abd-0437-4eca-8fd4-84e11119fb0b", job.ID())
	assert.NoError(t, job.Run())
	result, err := job.Result()
	assert.NoError(t, err)

	// Get the ETH price in USD
	ethPrice, err := getETHPriceInUSD()
	assert.NoError(t, err)
	ethPricString := fmt.Sprintf("%v", ethPrice)

	assert.Equal(t, result, []byte(ethPricString))
	t.Log(job.Payload())

	assert.Equal(t, DirectReqJob, job.Type())
	assert.Equal(t, "0x1234567890abcdef1234567890abcdef12345678", job.Payload().(DirectRequestTemplateParams).ContractAddress.String())
}

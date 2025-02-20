package jobs

import (
	"eavesdrop/crypto"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

type ChainID string

const (
	ChainIDEthereum ChainID = "ethereum"
	ChainIDSolana   ChainID = "Solana"
)

type DirectReqTask struct {
	Name   string  `toml:"name"`
	Type   string  `toml:"type"`
	Method string  `toml:"method,omitempty"`
	URL    string  `toml:"url,omitempty"`
	Input  string  `toml:"input,omitempty"`
	Path   string  `toml:"path,omitempty"`
	Factor float64 `toml:"factor,omitempty"`
}

type DirectRequestTemplateParams struct {
	ContractAddress   crypto.Address  `toml:"contractAddress"`
	EvmChainID        uint64          `toml:"evmChainID"`
	ChainID           ChainID         `toml:"chainID"`
	ExternalJobID     string          `toml:"externalJobId"`
	SchemaVersion     uint64          `toml:"schemaVersion"`
	Name              string          `toml:"name"`
	ObservationSource []DirectReqTask `toml:"observationSource"`
}

type DirectRequest struct {
	JobID   string
	JobType JobType
	params  DirectRequestTemplateParams
	Results map[string]interface{}
	timeout uint // timeout in ms
}

func (dr *DirectRequest) ID() string {
	return dr.JobID
}

func (dr *DirectRequest) Type() JobType {
	return dr.JobType
}

func (dr *DirectRequest) Payload() interface{} {
	return dr.params
}

func (dr *DirectRequest) Run() error {
	for _, task := range dr.params.ObservationSource {
		if err := dr.Execute(task); err != nil {
			return fmt.Errorf("error executing task %s: %v", task.Name, err)
		}
	}
	return nil
}

func (dr *DirectRequest) Result() ([]byte, error) {
	finalTask := dr.params.ObservationSource[len(dr.params.ObservationSource)-1]
	log.Printf("results, %v", dr.Results)
	result, ok := dr.Results[finalTask.Name]
	log.Printf("Results: %v", result)
	if !ok {
		return nil, fmt.Errorf("task %s not found in results", finalTask.Name)
	}
	return []byte(fmt.Sprintf("%v", result)), nil
}

func (dr *DirectRequest) TaskTimeout() time.Duration {
	return time.Duration(dr.timeout)
}

// Execute a task based on its type
func (dr *DirectRequest) Execute(task DirectReqTask) error {
	switch task.Type {
	case "http":
		resp, err := http.Get(task.URL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		dr.Results[task.Name] = string(body)

	case "json":
		input, ok := dr.Results[task.Input].(string)
		if !ok {
			return fmt.Errorf("invalid input for JSON parsing: %s", task.Input)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(input), &parsed); err != nil {
			return err
		}

		value, exists := parsed[task.Path]
		if !exists {
			return fmt.Errorf("key %s not found in JSON", task.Path)
		}

		dr.Results[task.Name] = value

	case "math":
		input, ok := dr.Results[task.Input].(float64)
		if !ok {
			if str, isStr := dr.Results[task.Input].(string); isStr {
				num, err := strconv.ParseFloat(str, 64)
				if err != nil {
					return fmt.Errorf("failed to convert input to float: %v", err)
				}
				input = num
			} else {
				return fmt.Errorf("invalid input for math operation: %s", task.Input)
			}
		}

		dr.Results[task.Name] = input * task.Factor

	case "log":
		fmt.Printf("Task %s output: %v\n", task.Name, dr.Results[task.Input])

	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}

	return nil
}

func (dr *DirectRequest) Listen(chan JobEventResponse) {
	// no-op
}

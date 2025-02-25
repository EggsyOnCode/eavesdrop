package jobs

import (
	"context"
	"eavesdrop/crypto"
	"eavesdrop/logger"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type ChainID string
type ReportingStrategy string

const (
	ChainIDEthereum ChainID = "ethereum"
	ChainIDSolana   ChainID = "Solana"

	HashCompare              ReportingStrategy = "hashcompare"
	MeanCalc                 ReportingStrategy = "mean"
	MultiFieldMajorityVoting ReportingStrategy = "fieldvote"
)

type DirectReqReportGenParms struct {
	Strategy ReportingStrategy `toml:"strategy"`
}

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
	ContractAddress   crypto.Address          `toml:"contractAddress"`
	EvmChainID        uint64                  `toml:"evmChainID"`
	ChainID           ChainID                 `toml:"chainID"`
	ExternalJobID     string                  `toml:"externalJobId"`
	SchemaVersion     uint64                  `toml:"schemaVersion"`
	Name              string                  `toml:"name"`
	ObservationSource []DirectReqTask         `toml:"observationSource"`
	ReportingStrategy DirectReqReportGenParms `toml:"reporting"`
}

type DirectRequest struct {
	JobID   string
	JobType JobType
	params  DirectRequestTemplateParams

	Results map[string]interface{}
	timeout uint // timeout in ms
	ReportAssembler

	mu sync.Mutex
}

func NewDirectReq(params DirectRequestTemplateParams) *DirectRequest {
	return &DirectRequest{
		JobID:   params.ExternalJobID,
		JobType: DirectReqJob,
		params:  params,
		Results: make(map[string]interface{}),
		timeout: 0,
		mu:      sync.Mutex{},
	}
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

func (dr *DirectRequest) Run(ctx context.Context) error {
	for _, task := range dr.params.ObservationSource {
		select {
		case <-ctx.Done():
			return fmt.Errorf("execution cancelled: %v", ctx.Err())
		default:
			// Execute the task and store results in `dr.Results`
			if err := dr.Execute(task); err != nil {
				return fmt.Errorf("error executing task %s: %v", task.Name, err)
			}
		}
	}

	// Select reporting strategy
	reporter, err := NewReportAssembler(dr.params.ReportingStrategy.Strategy)
	if err != nil {
		logger.Get().Sugar().Errorf("failed to create report assembler: %v", err)
		return err
	}

	dr.ReportAssembler = reporter

	return nil
}

func (dr *DirectRequest) Result() ([]byte, error) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

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
	dr.mu.Lock()
	defer dr.mu.Unlock()

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

		log.Printf("http result is %v", string(body))

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

		log.Printf("json result is %v", value)

		dr.Results[task.Name] = value

	case "math":
		input, ok := dr.Results[task.Input].(float64)
		log.Printf("input val is %v", input)
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

		log.Printf("math result is %v", input*task.Factor)

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

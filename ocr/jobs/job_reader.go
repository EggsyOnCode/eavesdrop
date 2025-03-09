package jobs

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"
)

type JobFormat byte
type ReadFrom string

const (
	JobFormatJSON JobFormat = iota
	JobFormatYAML JobFormat = 0x1
	JobFormatTOML JobFormat = 0x2

	ReadFromFs   ReadFrom = "fs"
	ReadFromHTTP ReadFrom = "http"
	ReadFromDB   ReadFrom = "db"
)

type JobReader interface {
	Read(io.Reader, JobReaderConfig) (*Job, error)
}

type JobReaderFactory struct {
	logger *zap.SugaredLogger
}
type JobReaderConfig struct {
	JobFormat
}

type JobTypeToml struct {
	Type string `toml:"type"`
}

func NewJobReaderFactory() *JobReaderFactory {
	return &JobReaderFactory{
		logger: zap.S().Named("job_reader_factory"),
	}
}

func (f *JobReaderFactory) Read(r io.Reader, c JobReaderConfig) (Job, error) {
	var jobType JobTypeToml
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, &jobType); err != nil {
		return nil, err
	}
	jType := JobType(jobType.Type)

	switch c.JobFormat {
	case JobFormatTOML:
		switch jType {
		case DirectReqJob:
			var drParams DirectRequestTemplateParams
			if err := toml.Unmarshal(data, &drParams); err != nil {
				return nil, err
			}

			var id string
			if drParams.ExternalJobID == "" {
				id = drParams.ContractAddress.String()
			} else {
				id = drParams.ExternalJobID
			}

			directReq := &DirectRequest{
				params:  drParams,
				JobID:   id,
				JobType: DirectReqJob,
				Results: make(map[string]interface{}),
			}

			return directReq, nil

		default:
			f.logger.Errorf("unsupported job type: %v", jType)
		}

	default:
		f.logger.Errorf("unsupported job format: %v", c.JobFormat)
	}

	return nil, nil
}

func ReadJobSpec(from ReadFrom, path string) (io.Reader, error) {
	switch from {
	case ReadFromFs:
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return bufio.NewReader(f), nil

	default:
		return nil, fmt.Errorf("unsupported read from: %v", from)
	}
}

func ReadJobsFromDir(dir string) ([]io.Reader, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var readers []io.Reader
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := filepath.Join(dir, file.Name()) // Construct full path
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		readers = append(readers, bufio.NewReader(f))
	}

	return readers, nil
}

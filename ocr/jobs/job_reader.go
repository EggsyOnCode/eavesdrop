package jobs

import (
	"io"

	toml "github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"
)

type JobFormat byte

const (
	JobFormatJSON JobFormat = iota
	JobFormatYAML JobFormat = 0x1
	JobFormatTOML JobFormat = 0x2
)

type JobReader interface {
	Read(io.Reader, JobReaderConfig) (*Job, error)
}

type JobReaderFactory struct {
	logger *zap.SugaredLogger
}
type JobReaderConfig struct {
	JobFormat
	JobType
}

func NewJobReaderFactory() *JobReaderFactory {
	return &JobReaderFactory{
		logger: zap.S().Named("job_reader_factory"),
	}
}

func (f *JobReaderFactory) Read(r io.Reader, c JobReaderConfig) (Job, error) {
	switch c.JobFormat {
	case JobFormatTOML:
		switch c.JobType {
		case DirectReqJob:
			data, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
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
			f.logger.Errorf("unsupported job type: %v", c.JobType)
		}

	default:
		f.logger.Errorf("unsupported job format: %v", c.JobFormat)
	}

	return nil, nil
}

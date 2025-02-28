package jobs

type JobSourceType string

const (
	JobSourceDir     JobSourceType = "dir"
	JobSourceDB      JobSourceType = "db"
	JobSourceNetwork JobSourceType = "network"
)

type JobSourceConfig struct {
	SourceType JobSourceType
	DirPath    string
	APIURL     string
}

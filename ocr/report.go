package ocr

import (
	"eavesdrop/crypto"
	"eavesdrop/rpc"
)

type Report struct {
	from    string
	reports rpc.JobReports
	sig     crypto.Signature
}

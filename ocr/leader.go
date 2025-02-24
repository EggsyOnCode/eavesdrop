package ocr

import (
	"crypto/sha256"
	"eavesdrop/ocr/jobs"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"encoding/json"
	"time"
)

type LeaderPhase byte

const (
	PhaseNil     LeaderPhase = 0xff
	PhaseObserve LeaderPhase = 0x0
	PhaseGrace   LeaderPhase = 0x1
	PhaseReport  LeaderPhase = 0x2
	PhaseFinal   LeaderPhase = 0x3

	DURATION_BW_OM_REPORT_REQ int = 5
)

type Observation []byte

type LeaderState struct {
	// map of peerIds to job responses
	observations      ObservationSafeMap // signed observations received in OBSERVE messages
	finalReport       *rpc.FinalReport   // final report to be sent in FINAL message
	reports           []Report           // attested reports received in REPORT messages
	TimerRoundTimeout *Timer             // timer Tround with timeout duration ∆round , initially stopped
	TimerGrace        *Timer             // timer Tgrace with timeout duration ∆grace , initially stopped
	Phase             LeaderPhase        // current phase of the leader
	phaseCh           chan LeaderPhase
}

func (re *ReportingEngine) handleObserve() {
	// handle the OBSERVE phase
	// will have to listen to the OBSERVE messages from the leader
	// and update the state accordingly

	// increment hte round number
	re.curRound++

	// get the list of jobs to be processed in nextRound
	curRoundJobInfos := re.GetJobInfosForCurrRound()

	// send out OBSERVE-REQ msgs and init the observations and reports if not already done
	observeReqMsg := rpc.ObserveReq{
		Epoch:  re.epoch,
		Round:  uint64(re.curRound),
		Leader: re.leader,
		Jobs:   curRoundJobInfos,
	}

	// constructing msg
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(re.serverOpts.Addr),
		re.serverOpts.Codec,
		re.serverOpts.ID,
	).SetHeaders(
		rpc.MessageObserveReq,
	).SetTopic(
		rpc.Reporter,
	).SetPayload(observeReqMsg).Bytes()

	if err != nil {
		panic(err)
	}

	re.msgService.BroadcastMsg(rpcMsg)

	// start the round timer (have a handler for the timer expiry) in top most loop
	re.LeaderState.TimerRoundTimeout = NewTimer(RoundTmeout)

	// change state to OBSERVE
	re.Phase = PhaseObserve
	re.phaseCh <- PhaseObserve
}

func (re *ReportingEngine) handleGrace() {
	// handle the GRACE phase
	// will have to listen to the GRACE messages from the leader
	// and update the state accordingly
	time.AfterFunc(RoundGrace, func() {
		re.logger.Infof("RE: GRACE phase ended, proceeding to next step...")

		re.Phase = PhaseReport
		re.LeaderState.phaseCh <- PhaseReport
	})

}

func (re *ReportingEngine) handleReport() {
	// handle the REPORT phase as a leader

	// braodcast JSON marshalled ObservationMap to all followers
	data, err := re.observations.MarshalJSON()
	if err != nil {
		re.logger.Errorf("RE: failed to marshal observations: %v", err)
		return
	}

	// first broadcast OM to all peers
	if err := re.broadcastObservationMap(data); err != nil {
		re.logger.Errorf("RE: failed to broadcast ObservationMap: %v", err)
		return
	}

	// sleep to allow msg to be braodcsted thruought the network
	time.Sleep(time.Duration(DURATION_BW_OM_REPORT_REQ) * time.Second)

	// construct the REPORT-REQ msg
	reportReq := rpc.ReportReq{
		Epoch:        re.epoch,
		Leader:       re.leader,
		Round:        uint64(re.curRound),
		Observations: data,
	}

	// constructing msg
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(re.serverOpts.Addr),
		re.serverOpts.Codec,
		re.serverOpts.ID,
	).SetHeaders(
		rpc.MessageReportReq,
	).SetTopic(
		rpc.Reporter,
	).SetPayload(reportReq).Bytes()

	if err != nil {
		panic(err)
	}

	re.msgService.BroadcastMsg(rpcMsg)

}

func (re *ReportingEngine) handleFinal() {
	// handle the FINAL phase
	// will have to listen to the FINAL messages from the leader
	// and update the state accordingly

	// construct and sign BroadcastFinalReport msg to all followers
	if re.finalReport == nil {
		re.logger.Errorf("RE: final report is empty, cannot proceed to FINAL phase")
		return
	}
	finalRepMsg := rpc.BroadcastFinalReport{
		Epoch:       re.epoch,
		Leader:      re.leader,
		Round:       uint64(re.curRound),
		FinalReport: *re.finalReport,
	}

	// signing observeMsg
	msgBytes, err := finalRepMsg.Bytes(re.serverOpts.Codec)
	if err != nil {
		re.logger.Errorf("RE: err converting to bytes")
		return
	}

	signature, err := re.SignMessage(msgBytes)
	if err != nil {
		re.logger.Errorf("RE: err signing msg bytes")
		return
	}

	// constructing msg
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(re.serverOpts.Addr),
		re.serverOpts.Codec,
		re.serverOpts.ID,
	).SetHeaders(
		rpc.MessageReportReq,
	).SetTopic(
		rpc.Reporter,
	).SetPayload(finalRepMsg).SetSignature(signature).Bytes()

	if err != nil {
		panic(err)
	}

	re.msgService.BroadcastMsg(rpcMsg)
}

func (re *ReportingEngine) GetJobInfosForCurrRound() []jobs.JobInfo {
	// get the list of jobs for the current round
	js := re.jobSchedule[re.curRound]
	var jobInfos []jobs.JobInfo
	for _, job := range js {
		jobInfo := jobs.JobInfo{
			JobID:    job.ID(),
			JobType:  job.Type(),
			Template: job.Payload(),
			Timeout:  job.TaskTimeout(),
		}

		jobInfos = append(jobInfos, jobInfo)
	}

	return jobInfos
}

func (re *ReportingEngine) broadcastObservationMap(data []byte) error {
	// construct the BraodcastObservationMap rpc message
	observeMap := rpc.BroadcastObservationMap{
		Epoch:        re.epoch,
		Leader:       re.leader,
		Round:        uint64(re.curRound),
		Observations: data,
	}

	// constructing msg
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(re.serverOpts.Addr),
		re.serverOpts.Codec,
		re.serverOpts.ID,
	).SetHeaders(
		rpc.MessageObservationMap,
	).SetTopic(
		rpc.Reporter,
	).SetPayload(observeMap).Bytes()

	if err != nil {
		return err
	}

	re.msgService.BroadcastMsg(rpcMsg)

	return nil
}

func (re *ReportingEngine) assembleFinalReport() (*rpc.FinalReport, bool) {
	// will be called by the leader
	// compare hashes of all the reports obtained in more than f+1 REPORT_RES msgs
	// if for f+1/n nodes, the hash is same, then the final report is valid
	// if not, then the final report is invalid and move to next round
	// final report consists of Data and at least f+1 signatories

	jobHashCount := make(map[string]map[[32]byte]int)               // jobID -> (hash -> count)
	jobFinalReports := make(map[string]map[[32]byte]rpc.JobReports) // jobID -> (hash -> final report)
	hashSignatories := make(map[[32]byte][]rpc.Signatories)         // hash -> list of signatories

	// Iterate over all received reports
	for _, report := range re.reports {
		for jobID, response := range report.reports { // Iterate over jobID -> responseData
			// Serialize responseData to bytes for hashing
			responseBytes, err := json.Marshal(response)
			if err != nil {
				re.logger.Errorf("Failed to serialize response for job %s: %v", jobID, err)
				continue
			}

			// Compute hash of response
			hash := sha256.Sum256(responseBytes)

			// Initialize job's hash count map if not present
			if jobHashCount[jobID] == nil {
				jobHashCount[jobID] = make(map[[32]byte]int)
				jobFinalReports[jobID] = make(map[[32]byte]rpc.JobReports)
			}

			// Increment count of this hash for the job
			jobHashCount[jobID][hash]++

			// Store the corresponding report for this hash
			jobFinalReports[jobID][hash] = report.reports

			// Store signatory (includes both public key & signature)
			signatory := rpc.Signatories{
				Sign: report.sig,
				ID:    report.from, // Public key of the signer
			}
			hashSignatories[hash] = append(hashSignatories[hash], signatory)
		}
	}

	// Threshold for valid consensus
	threshold := int(re.pacemakerGlobals.f) + 1

	// Construct final report
	finalJobReports := make(rpc.JobReports)
	finalSignatories := []rpc.Signatories{}

	for jobID, hashCounts := range jobHashCount {
		for hash, count := range hashCounts {
			if count >= threshold {
				// Found consensus for this job
				finalJobReports[jobID] = jobFinalReports[jobID][hash][jobID]

				// Collect f+1 valid signatories
				signatories := hashSignatories[hash]
				if len(signatories) >= threshold {
					finalSignatories = append(finalSignatories, signatories[:threshold]...)
				}
				break // Move to next jobID
			}
		}
	}

	// If no consensus reached
	if len(finalJobReports) == 0 || len(finalSignatories) < threshold {
		return nil, false
	}

	// Return FinalReport
	return &rpc.FinalReport{
		Report:      finalJobReports,
		Signatories: finalSignatories, // Include f+1 signatories
	}, true
}

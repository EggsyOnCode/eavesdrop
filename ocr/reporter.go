package ocr

import (
	"context"
	c "eavesdrop/crypto"
	"eavesdrop/ocr/jobs"
	"eavesdrop/rpc"
	"eavesdrop/utils"
	"fmt"
	"reflect"
	"sync"
	"time"

	fifo "github.com/foize/go.fifo"

	"go.uber.org/zap"
)

const (
	MAX_ROUNDS                = 10
	INIT_ROUND                = 1
	RoundTmeout time.Duration = time.Second * 20
	RoundGrace  time.Duration = time.Second * 5

	jobsDir string = "../jobs_def"
)

// info about the server (p2p network) that needs to be exposed to the reporting engine
type ServerInfo struct {
	Addr  string
	Codec rpc.Codec
	ID    string
}

// some globals from Pacemaker
type PacemakerGlobals struct {
	n uint // total number of oracles
	f uint // number of faulty oracles
}

// lasts only for a round and is reset after that
type CacheLayer struct {
	observe_n uint // no of observations received
	report_n  uint // no of reports received

	jobs                  []jobs.Job
	jobReports            rpc.JobReports
	follower_observations ObservationSafeMap // observation map broadcasted by the leader to all followers before REPORT_REQ msg
	c_finalReport         rpc.FinalReport    // final report broadcasted by the leader to all followers before FINAL msg
}

type ReportingEngine struct {
	curRound         int    // current round (shared state between leader and non-leading oracles)
	isLeader         bool   // indicates if the oracle is the leader or follower
	epoch            uint64 // current epoch number
	leader           string // current leader's PeerID
	LeaderState             // set either of the states, depending upon if hte oracle is leader or not
	msgService       MessagingLayer
	serverOpts       ServerInfo
	quitCh           chan struct{}
	logger           *zap.SugaredLogger
	cacheLayer       CacheLayer
	pacemakerGlobals PacemakerGlobals
	signer           c.PrivateKey
	recEvents        *fifo.Queue
	jobRegistry      *map[string]jobs.Job // used to query the job info when an event is received
	jobSchedule      map[int][]jobs.Job   // schedule of jobs to be observed in each round
}

func NewReportingEngine(isLeader bool, epoch uint64, leader string, s_info ServerInfo, p_globals PacemakerGlobals, signer c.PrivateKey) *ReportingEngine {
	re := &ReportingEngine{
		epoch:       epoch,
		serverOpts:  s_info,
		leader:      leader,
		jobSchedule: map[int][]jobs.Job{},
		signer:      signer,
		jobRegistry: &map[string]jobs.Job{},
		LeaderState: LeaderState{
			observations: ObservationSafeMap{},
			reports:      make([]Report, 1),
			// TimerRoundTimeout: NewTimer(RoundTmeout),
			TimerRoundTimeout: &Timer{},
			// TimerGrace:        NewTimer(RoundGrace),
			TimerGrace: &Timer{},
			Phase:      PhaseNil, // should only be set if leader
			phaseCh:    make(chan LeaderPhase),
		},
		curRound:         INIT_ROUND,
		isLeader:         isLeader,
		pacemakerGlobals: p_globals,
		cacheLayer: CacheLayer{
			observe_n: 0,
			report_n:  0,
		},
	}

	// config logger setup
	re.logger = zap.S().With("epoch", epoch, "leader", leader, "round", re.curRound)

	// if leader , do some intial config
	if isLeader {
		re.LeaderState.Phase = PhaseObserve // inital state
	}

	return re
}

func (re *ReportingEngine) Start(recEvents *fifo.Queue, jobReg *map[string]jobs.Job) {
	// function's design:
	// permanet listener, will use a quitch to stop the listener and current RE
	// if leader: will listen to channel for updates to re.phase and upon each phase
	// change, will start a new goroutine to handle the phase, those handlers will then process the rpc messages and update the state and phase as necessaryu
	// which will be then picked up by the main listener
	// if non-leader: will listen to rpc msgs from the leader and handle them accordingly
	// in fact a meta-seprator can be used where if hte re is a leader, launcha main listener in a go routine and then launch a phase handler in a go routine
	// if non-leader, launch a main listener in a separate go routine

	// pacemaker will read jobs from a reader (say a file / dir / networ)
	// and cast them into jobs.Job and return a map regsitry such that jobId -> Job
	re.jobRegistry = jobReg

	if re.isLeader {

		// schedule the jobs to be observed during each round of the epoch
		re.recEvents = recEvents // received from pacemaker
		n := recEvents.Len()
		jobs_per_round := (n / MAX_ROUNDS)
		for i := 0; i < MAX_ROUNDS; i++ {
			for j := 1; j < int(jobs_per_round); j++ {
				job := recEvents.Next()
				re.jobSchedule[i] = append(re.jobSchedule[i], job.(jobs.Job))
			}
		}

		go re.orchestrateLeadership()
		time.Sleep(1 * time.Second) // to ensure the leader is ready
		re.LeaderState.phaseCh <- PhaseObserve
	} else {
		go re.orchestrateFollowing()
	}
}

func (re *ReportingEngine) orchestrateLeadership() {
	// leader orchestration
	// will have to listen to the phase changes and then start a new goroutine to handle the phase
	// the phase handlers will then process the rpc messages and update the state and phase as necessary
	// which will be then picked up by the main listener

free:
	for {
		select {
		case phase := <-re.LeaderState.phaseCh:
			switch phase {
			case PhaseObserve:
				go re.handleObserve()
			case PhaseGrace:
				go re.handleGrace()
			case PhaseReport:
				go re.handleReport()
			case PhaseFinal:
				go re.handleFinal()
			case PhaseTransmit:
				go re.handleTransmit()
			}

		case <-re.quitCh:
			break free
		}
	}
}

func (re *ReportingEngine) orchestrateFollowing() {
	// non-leader orchestration
	// will listen to rpc msgs from the leader and handle them accordingly

free:
	for {
		select {
		case <-re.quitCh:
			break free
		}
	}
}

// msg expected is serialized version of RPCMessage
func (re *ReportingEngine) SendMsg(s SendingSchme, msg []byte, id string) {
	switch s {
	case BROADCAST:
		re.msgService.BroadcastMsg(msg)
	case LEADER:
		// will have to fetch leader info from Packemaker via channels
		re.msgService.SendMsg(re.leader, msg)
	case PEER:
		re.msgService.SendMsg(id, msg)
	}
}

func (re *ReportingEngine) AttachMsgLayer(msgService MessagingLayer) {
	re.msgService = msgService
}

// all messages with say topic Reporter will be routed to repoter.ProcessMessage, now this msgs could be of any type , like leader senidng msg, braodcsating something, an oracle submitting an observation (all of these msgs should be defiend in rpc_leader.go inside rpc pkg i think), so reporter.ProcessMessage could either handle all of them itself and update the state of the RE glovally or it could delegrate, for now, lets shouild build a monolithic RPC handler for reptoer inside ProcessMessgae
func (re *ReportingEngine) ProcessMessage(msg *rpc.DecodedMsg) error {
	// prechecks
	if res, err := re.rpcMsgPrechecks(msg); err != nil || !res {
		return fmt.Errorf("RE: rpc msg prechecks failed")
	}

	switch msg.Data.(type) {
	case rpc.ObserveReq:
		//shall only be recevied by followers
		msg := msg.Data.(rpc.ObserveReq)
		re.logger.Infof("RE: received OBSERVE-REQ msg: %v", msg)

		re.curRound = int(msg.Round)

		// ideally what shoudl be happening is this:
		// leader sends progressRound msg to all followers and increase its curRound count locally and chagnes phase to observe
		// during observe phase, it would send out observeReq msg to all followers
		// containing curRound that is > MAX_ROUNDS, it would stop the RE for the current epoch
		if re.curRound > MAX_ROUNDS {
			re.logger.Infof("RE: max rounds reached, stopping RE")

			// broadcast changeleader event (change leader and newepoch msg have the
			// same effect) i.e pacemaker will start a new epoch with new leader
			newEpoch := rpc.NewEpochMesage{}

			// constructing msg
			rpcMsg, err := rpc.NewRPCMessageBuilder(
				utils.NetAddr(re.serverOpts.Addr),
				re.serverOpts.Codec,
				re.serverOpts.ID,
			).SetHeaders(
				rpc.MessageNewEpoch,
			).SetTopic(
				rpc.Pacemaker,
			).SetPayload(newEpoch).Bytes()

			if err != nil {
				panic(err)
			}

			re.msgService.BroadcastMsg(rpcMsg)

			re.Stop()
		}

		var wg sync.WaitGroup
		resChan := make(chan jobs.JobObservationResponse, len(msg.Jobs))
		errChan := make(chan error, len(msg.Jobs))

		// Create job instances from jobInfos received
		for _, jobInfo := range msg.Jobs {
			job, err := jobs.NewJobFromInfo(jobInfo)
			if err != nil {
				re.logger.Errorf("RE: err creating job instance from info")
				return nil
			}

			// add jobInfo to cache layer , to be used later
			// for reporting
			re.cacheLayer.jobs = append(re.cacheLayer.jobs, job)

			wg.Add(1)
			go func(job jobs.Job, jobInfo jobs.JobInfo) {
				defer wg.Done()

				// Run the job with a timeout context
				ctx, cancel := context.WithTimeout(context.Background(), jobInfo.Timeout)
				defer cancel()

				if err := job.Run(ctx); err != nil {
					re.logger.Errorf("RE: err running job's observation: %v", err)
					errChan <- err
					return
				}

				// Fetch result
				res, err := job.Result()
				if err != nil {
					re.logger.Errorf("RE: err obtaining job's result: %v", err)
					errChan <- err
					return
				}

				// Send result to channel
				resChan <- jobs.JobObservationResponse{
					JobId:    jobInfo.JobID,
					Response: res,
				}
			}(job, jobInfo)
		}

		// Wait for all jobs to finish
		wg.Wait()
		close(resChan)
		close(errChan)

		// Collect all results
		var responses []jobs.JobObservationResponse
		for res := range resChan {
			responses = append(responses, res)
		}

		// If there were errors, return the first one
		if err := <-errChan; err != nil {
			return err
		}

		observeResmsg := rpc.ObserveResp{
			Epoch:        re.epoch,
			Leader:       re.leader,
			Round:        uint64(re.curRound),
			JobResponses: responses,
		}

		// signing observeMsg
		msgBytes, err := observeResmsg.Bytes(re.serverOpts.Codec)
		if err != nil {
			re.logger.Errorf("RE: err converting to bytes")
			return nil
		}

		signature, err := re.SignMessage(msgBytes)
		if err != nil {
			re.logger.Errorf("RE: err signing msg bytes")
			return nil
		}

		// constructing msg
		rpcMsg, err := rpc.NewRPCMessageBuilder(
			utils.NetAddr(re.serverOpts.Addr),
			re.serverOpts.Codec,
			re.serverOpts.ID,
		).SetHeaders(
			rpc.MessageObserveRes,
		).SetTopic(
			rpc.Reporter,
		).SetPayload(observeResmsg).SetSignature(signature).Bytes()

		if err != nil {
			panic(err)
		}

		// send back to leader
		re.msgService.SendMsg(re.leader, rpcMsg)

		return nil

	case rpc.ObserveResp:
		// shall only be received by teh leader

		observation := msg.Data.(rpc.ObserveResp)
		if (observation.Round == uint64(re.curRound)) && (observation.Epoch == re.epoch) {
			// verify the msg signature
			msgBytes, err := observation.Bytes(re.serverOpts.Codec)
			if err != nil {
				re.logger.Errorf("RE: err converting to bytes")
				return nil
			}

			var pk c.PublicKey
			if pk, err = c.StringToPublicKey(msg.FromId); err != nil {
				re.logger.Errorf("RE: err casting strng to pk")
				return nil
			}

			isSigned := msg.Signature.Verify(msgBytes, pk)
			if isSigned {
				// add observations to the state
				observationMap := make(map[string][]jobs.JobObservationResponse) // ✅ Initialize the map

				for _, j := range observation.JobResponses {
					observationMap[j.JobId] = append(observationMap[j.JobId], j) // ✅ No nil dereference
				}

				re.observations.Store(msg.FromId, observationMap)

			} else {
				re.logger.Errorf("RE: observe res msg incorreclty / not signed; dropping..")
				return nil
			}

			// update the cache layer
			re.cacheLayer.observe_n++

			if re.cacheLayer.observe_n > 2*re.pacemakerGlobals.f+1 {
				re.Phase = PhaseGrace
				re.LeaderState.phaseCh <- PhaseGrace
			}
		}

		return nil
	case rpc.BroadcastObservationMap:
		// received by followers from leader

		// Step 1: Extract and Unmarshal Observations
		observationMap := msg.Data.(rpc.BroadcastObservationMap).Observations

		var obsMap ObservationSafeMap
		if err := obsMap.UnmarshalJSON(observationMap); err != nil {
			re.logger.Errorf("RE: error unmarshalling observation map: %v", err)
			return err
		}

		// Step 2: Persist observations in cache layer
		// TODO: fix issue: assignment copies lock value to re.cacheLayer.follower_observations: eavesdrop/ocr.ObservationSafeMap contains sync.Map contains sync.Mutex
		re.cacheLayer.follower_observations = obsMap

		return nil

	case rpc.ReportReq:
		// will be received by followers

		// some docs: leader sends us observations of all the
		// jobs for current round for all peers
		// []{peerID: []{jobID: observed_value}}

		// what follower needs to send is the final reporting value of the each job
		// []{jobID: finalValue}

		// Step 1: Extract and Unmarshal Observations
		observationMap := msg.Data.(rpc.ReportReq).Observations

		var obsMap ObservationSafeMap
		if err := obsMap.UnmarshalJSON(observationMap); err != nil {
			re.logger.Errorf("RE: error unmarshalling observation map: %v", err)
			return err
		}

		// compare OM recevied in the reportReq with the OM broadcasted by the leader
		cacheObsData, _ := re.cacheLayer.follower_observations.MarshalJSON()
		if !utils.CompareHashes([][]byte{observationMap, cacheObsData}) {
			re.logger.Errorf("RE: OM in reportReq and broadcasted OM do not match")
			return nil
		}

		// Step 2: Reorganize observations by jobID
		jobObservations := make(map[string][]jobs.JobObservationResponse) // jobID -> all peer observations

		obsMap.Iterate(func(peerID string, peerObservations map[string][]jobs.JobObservationResponse) bool {
			for jobID, observations := range peerObservations {
				jobObservations[jobID] = append(jobObservations[jobID], observations...)
			}
			return true // Continue iteration
		})

		// Step 3: Process observations and generate final report
		jobReportValues := make(rpc.JobReports) // jobID -> final reported value

		for jobID, observations := range jobObservations {
			job := (*re.jobRegistry)[jobID]
			finalValue, err := job.Assemble(observations)
			if err != nil {
				re.logger.Errorf("RE: error assembling job %s: %v", jobID, err)
				continue // Skip problematic job
			}
			jobReportValues[jobID] = finalValue
		}

		// persist in cache layer
		re.cacheLayer.jobReports = jobReportValues

		// Step 4: Send report back to the leader
		reportRes := rpc.ReportRes{
			Epoch:   re.epoch,
			Round:   uint64(re.curRound),
			Leader:  re.leader,
			Reports: jobReportValues,
		}

		// signing reportMsg
		msgBytes, err := reportRes.Bytes(re.serverOpts.Codec)
		if err != nil {
			re.logger.Errorf("RE: err converting to bytes")
			return nil
		}

		signature, err := re.SignMessage(msgBytes)
		if err != nil {
			re.logger.Errorf("RE: err signing msg bytes")
			return nil
		}

		// constructing msg
		rpcMsg, err := rpc.NewRPCMessageBuilder(
			utils.NetAddr(re.serverOpts.Addr),
			re.serverOpts.Codec,
			re.serverOpts.ID,
		).SetHeaders(
			rpc.MessageReportRes,
		).SetTopic(
			rpc.Reporter,
		).SetPayload(reportRes).SetSignature(signature).Bytes()

		if err != nil {
			panic(err)
		}

		re.msgService.SendMsg(re.leader, rpcMsg)

		return nil

	case rpc.ReportRes:
		// will be received by the leader (from the followers)
		reports := msg.Data.(rpc.ReportRes)

		// verify the msg signature
		msgBytes, err := reports.Bytes(re.serverOpts.Codec)
		if err != nil {
			re.logger.Errorf("RE: err converting to bytes")
			return nil
		}

		var pk c.PublicKey
		if pk, err = c.StringToPublicKey(msg.FromId); err != nil {
			re.logger.Errorf("RE: err casting strng to pk")
			return nil
		}

		isSigned := msg.Signature.Verify(msgBytes, pk)
		if isSigned {
			// add reports to the leader state
			report := Report{
				sig:     msg.Signature,
				from:    msg.FromId,
				reports: reports.Reports,
			}
			re.reports = append(re.reports, report)

			// we have received n reports; byzantine tolernace surpassed
			// we can't assemble report after just f+1, since if the hashes are different, we need to move to the next round
			// we are wiating for all responses so taht if any f+1/n agree on the report, we can assemble the final report
			if len(re.reports) >= int(re.pacemakerGlobals.n) {
				// cReached -> consensus reached
				finalR, cReached := re.assembleFinalReport()
				if cReached {
					re.finalReport = finalR
					// phase change
					re.Phase = PhaseFinal
					re.LeaderState.phaseCh <- PhaseFinal
				} else {
					// TODO: move to next round
				}
			}

		} else {
			re.logger.Errorf("RE: report res msg incorreclty / not signed; dropping..")
			return nil
		}

		return nil

	case rpc.BroadcastFinalReport:
		// Only followers receive this

		// Step 1: Extract and Unmarshal Final Report
		finalReportMsg := msg.Data.(rpc.BroadcastFinalReport)

		msgBytes, err := finalReportMsg.Bytes(re.serverOpts.Codec)
		if err != nil {
			re.logger.Errorf("RE: err converting to bytes")
			return nil
		}

		// Step 2: Verify leader's signature
		isSigned, err := VerifySignature(msgBytes, msg.FromId, msg.Signature)
		if err != nil || !isSigned {
			re.logger.Errorf("RE: err verifying leader's signature")
			return nil
		}

		// Step 3: Verify f+1 signatures from validators
		validSignatures := 0
		threshold := int(re.pacemakerGlobals.f) + 1
		uniqueValidators := make(map[string]bool) // Track unique signers

		for _, signatory := range finalReportMsg.Signatories {
			pk, err := c.StringToPublicKey(signatory.ID)
			if err != nil {
				re.logger.Errorf("RE: err converting string to public key")
				return nil
			}

			// also verify if signatory is a valid peer in our network
			if !re.msgService.IsPeer(signatory.ID) {
				re.logger.Errorf("RE: signatory is not a valid peer")
				return nil
			}

			if signatory.Sign.Verify(msgBytes, pk) {

				// Ensure unique validators are counted
				if !uniqueValidators[signatory.ID] {
					uniqueValidators[signatory.ID] = true
					validSignatures++
				}
			}

			// Stop early if we reach f+1 valid signatures
			if validSignatures >= threshold {
				break
			}
		}

		if validSignatures < threshold {
			re.logger.Errorf("RE: insufficient valid signatures, expected at least %d but got %d", threshold, validSignatures)
			return nil
		}

		re.logger.Infof("RE: Final report verified successfully with %d signatures", validSignatures)

		// Step 4: Persist final report in cache layer
		re.cacheLayer.c_finalReport = finalReportMsg.FinalReport

		// send a FINAL echo back to leader
		if err := re.sendFinalReportEcho(); err != nil {
			re.logger.Errorf("RE: err sending final report echo")
			return nil
		}

		return nil

	case rpc.FinalEcho:
		// received by the leader only

		finalEcho := msg.Data.(rpc.FinalEcho)
		if (finalEcho.Epoch != re.epoch) || (finalEcho.Round != uint64(re.curRound) || (finalEcho.Leader != re.leader)) {
			re.logger.Errorf("RE: final echo msg for wrong epoch or round")
			return nil
		}

		// verify the msg signature and check valid peer
		msgBytes, err := finalEcho.Bytes(re.serverOpts.Codec)
		if err != nil {
			re.logger.Errorf("RE: err converting to bytes")
			return nil
		}
		signed := re.VerifySigAndPeer(msgBytes, msg.FromId, msg.Signature)
		if signed {
			re.finalReportAttestation++

			if re.finalReportAttestation >= uint(re.pacemakerGlobals.f)+1 {
				// change phase to Transmittion
				re.Phase = PhaseTransmit
				re.LeaderState.phaseCh <- PhaseTransmit
			}
		} else {
			re.logger.Errorf("RE: final echo msg not signed or not a valid peer")
			return nil
		}

		return nil

	case rpc.ProgressRound:
		// received by the follower only only

		progressMsg := msg.Data.(rpc.ProgressRound)

		msgBytes, err := progressMsg.Bytes(re.serverOpts.Codec)
		if err != nil {
			re.logger.Errorf("RE: err converting to bytes")
			return err
		}

		// verify the msg signature if of the leader
		isSigned, err := VerifySignature(msgBytes, re.leader, msg.Signature)
		if err != nil || !isSigned {
			re.logger.Errorf("RE: err verifying leader's signature")
			return fmt.Errorf("RE: err verifying leader's signature")
		}

		// if signed, update the state and move to the next round
		go re.progressToNextRound()

		return nil
	default:
		re.logger.Errorf("RE: unknown message type: %v", reflect.TypeOf(msg.Data))
		return nil
	}
}

// to be called by the pacemaker, should return the imp state of current
// epoch needed to bootstrap the new epoch
// TODO : implement this
func (r *ReportingEngine) Stop() {
	// stop the reporting engine
	close(r.quitCh)
	// cleanup the state
	r.cleanupFunc()
}

func (re *ReportingEngine) cleanupFunc() {
	// Ensure cleanup happens only once
	re.logger.Info("Cleaning up ReportingEngine resources...")

	// Close quit channel to signal shutdown
	select {
	case <-re.quitCh:
		// Already closed
	default:
		close(re.quitCh)
	}

	// Close phase channel if leader
	if re.isLeader {
		close(re.phaseCh)
	}

	// Stop timers if they exist
	if re.TimerRoundTimeout != nil {
		re.TimerRoundTimeout.Stop()
	}
	if re.TimerGrace != nil {
		re.TimerGrace.Stop()
	}

	// Clear job schedule and registry
	re.jobSchedule = nil
	re.jobRegistry = nil

	// Reset cache layer
	re.cacheLayer.observe_n = 0
	re.cacheLayer.report_n = 0

	// Clear received events queue
	if re.recEvents != nil {
		re.recEvents = nil
	}

	// Nullify messaging service reference
	re.msgService = nil

	// Final log before complete shutdown
	re.logger.Info("ReportingEngine cleanup complete.")
}

func (re *ReportingEngine) SignMessage(msg []byte) (c.Signature, error) {
	sign, err := re.signer.Sign(msg)
	return *sign, err
}

// VerifySignature checks if the given message's signature is valid.
func VerifySignature(msgBytes []byte, fromID string, signature c.Signature) (bool, error) {
	// Convert the string ID to a PublicKey
	pk, err := c.StringToPublicKey(fromID)
	if err != nil {
		return false, fmt.Errorf("failed to convert string to public key")
	}

	// Verify the signature
	if !signature.Verify(msgBytes, pk) {
		return false, fmt.Errorf("signature verification failed")
	}

	return true, nil
}

func (re *ReportingEngine) sendFinalReportEcho() error {

	// Step 4: Send report back to the leader
	reportRes := rpc.FinalEcho{
		Epoch:  re.epoch,
		Round:  uint64(re.curRound),
		Leader: re.leader,
	}

	// signing reportMsg
	msgBytes, err := reportRes.Bytes(re.serverOpts.Codec)
	if err != nil {
		re.logger.Errorf("RE: err converting to bytes")
		return err
	}

	signature, err := re.SignMessage(msgBytes)
	if err != nil {
		re.logger.Errorf("RE: err signing msg bytes")
		return err
	}

	// constructing msg
	rpcMsg, err := rpc.NewRPCMessageBuilder(
		utils.NetAddr(re.serverOpts.Addr),
		re.serverOpts.Codec,
		re.serverOpts.ID,
	).SetHeaders(
		rpc.MessageFinalEcho,
	).SetTopic(
		rpc.Reporter,
	).SetPayload(reportRes).SetSignature(signature).Bytes()

	if err != nil {
		return err
	}

	re.msgService.SendMsg(re.leader, rpcMsg)

	return nil
}

func (re *ReportingEngine) VerifySigAndPeer(msg []byte, fromID string, sign c.Signature) bool {
	// convert the string ID to a PublicKey
	pk, err := c.StringToPublicKey(fromID)
	if err != nil {
		return false
	}

	// verify the signature
	ifSigned := sign.Verify(msg, pk)

	var isPeer bool
	// check if the peer is a valid peer in our network
	if !re.msgService.IsPeer(fromID) {
		isPeer = false
	}

	return ifSigned && isPeer

}

func (re *ReportingEngine) progressToNextRound() {
	// 1. reset cache layer
	// 2. reset leader state pertaining to currRound
	// 3. update currRound
	// 4. update phase to Observe

	// if node is the leader, reset the leader state
	if re.serverOpts.ID == re.leader {
		re.LeaderState = LeaderState{
			observations:           ObservationSafeMap{},
			finalReport:            &rpc.FinalReport{},
			reports:                make([]Report, 1),
			Phase:                  PhaseObserve,
			finalReportAttestation: 0,
			phaseCh:                make(chan LeaderPhase),
		}
	} else {
		re.cacheLayer = CacheLayer{
			observe_n:             0,
			report_n:              0,
			jobs:                  []jobs.Job{},
			jobReports:            rpc.JobReports{},
			follower_observations: ObservationSafeMap{},
			c_finalReport:         rpc.FinalReport{},
		}
	}

	re.curRound++
}

func (re *ReportingEngine) rpcMsgPrechecks(msg *rpc.DecodedMsg) (bool, error) {
	// if the msg recived has a phase number different from the current phase, then ignore the msg
	// also if the msg's curr round or epoch is diff, handle that accordingly

	arbMsg := msg.Data
	if epoch, round, leader, phase, err := getCommonFieldsAndPhase(arbMsg); err != nil {
		return false, err
	} else {
		if (epoch != re.epoch) || (round != uint64(re.curRound)) || (leader != re.leader) || re.Phase != phase {
			return false, fmt.Errorf("RE: msg for wrong epoch or round or phase or leader")
		}
	}

	return true, nil
}

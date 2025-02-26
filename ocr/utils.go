package ocr

import (
	"eavesdrop/rpc"
	"errors"
	"reflect"
)

var typeToPhase = map[reflect.Type]LeaderPhase{
	reflect.TypeOf(rpc.ObserveReq{}):              PhaseObserve,
	reflect.TypeOf(rpc.ObserveResp{}):             PhaseObserve,
	reflect.TypeOf(rpc.ReportReq{}):               PhaseReport,
	reflect.TypeOf(rpc.ReportRes{}):               PhaseReport,
	reflect.TypeOf(rpc.BroadcastObservationMap{}): PhaseReport,
	reflect.TypeOf(rpc.BroadcastFinalReport{}):    PhaseFinal,
	reflect.TypeOf(rpc.FinalEcho{}):               PhaseFinal,
}

// Generic function to extract common fields and determine phase
func getCommonFieldsAndPhase(data interface{}) (uint64, uint64, string, LeaderPhase, error) {
	v := reflect.ValueOf(data)
	t := reflect.TypeOf(data)

	// Ensure it's a struct or pointer to struct
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0, 0, "", PhaseNil, errors.New("data is not a struct")
	}

	// Extract common fields
	epoch := v.FieldByName("Epoch")
	round := v.FieldByName("Round")
	leader := v.FieldByName("Leader")

	// Check if fields exist
	if !epoch.IsValid() || !round.IsValid() || !leader.IsValid() {
		return 0, 0, "", PhaseNil, errors.New("required fields missing")
	}

	// Get the phase from type map
	phase, exists := typeToPhase[t]
	if !exists {
		return 0, 0, "", PhaseNil, errors.New("unknown message type")
	}

	return epoch.Uint(), round.Uint(), leader.String(), phase, nil
}

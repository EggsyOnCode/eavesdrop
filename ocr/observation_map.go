package ocr

import (
	"eavesdrop/ocr/jobs"
	"encoding/json"
	"sync"
)

// map of peerId to a map of observatins for diff jobIds
type ObservationSafeMap struct {
	store sync.Map
}

// Store a value in the map
func (o *ObservationSafeMap) Store(key string, value map[string][]jobs.JobObservationResponse) {
	o.store.Store(key, value)
}

// Load a value from the map
func (o *ObservationSafeMap) Load(key string) (map[string][]jobs.JobObservationResponse, bool) {
	value, ok := o.store.Load(key)
	if !ok {
		return nil, false
	}
	obs, ok := value.(map[string][]jobs.JobObservationResponse)
	return obs, ok
}

// Iterate over all key-value pairs
func (o *ObservationSafeMap) Iterate(callback func(key string, value map[string][]jobs.JobObservationResponse) bool) {
	o.store.Range(func(k, v any) bool {
		key, ok := k.(string)
		if !ok {
			return true // Continue iteration
		}
		value, ok := v.(map[string][]jobs.JobObservationResponse)
		if !ok {
			return true // Continue iteration
		}
		return callback(key, value) // User-defined callback controls iteration
	})
}

// Marshal the ObservationSafeMap into JSON
func (o *ObservationSafeMap) MarshalJSON() ([]byte, error) {
	tempMap := make(map[string]map[string][]jobs.JobObservationResponse)

	o.store.Range(func(k, v any) bool {
		key, ok := k.(string)
		if !ok {
			return true // Skip invalid keys
		}
		value, ok := v.(map[string][]jobs.JobObservationResponse)
		if !ok {
			return true // Skip invalid values
		}
		tempMap[key] = value
		return true // Continue iteration
	})

	return json.Marshal(tempMap) // Convert to JSON
}

// UnmarshalJSON converts JSON data into an ObservationSafeMap
func (o *ObservationSafeMap) UnmarshalJSON(data []byte) error {
	// Temporary map to hold unmarshalled data
	tempMap := make(map[string]map[string][]jobs.JobObservationResponse)

	// Unmarshal into temp map
	if err := json.Unmarshal(data, &tempMap); err != nil {
		return err
	}

	// Store values in sync.Map
	for key, value := range tempMap {
		o.store.Store(key, value)
	}

	return nil
}

package rpc

type NewEpochMesage struct {
	EpochID uint64
}

func (newEpoch *NewEpochMesage) Bytes(c Codec) ([]byte, error) {
	return c.Encode(newEpoch)
}

type ChangeLeaderMessage struct{}

func (clm *ChangeLeaderMessage) Bytes(c Codec) ([]byte, error) {
	return c.Encode(clm)
}

package rpc

type CodecType byte

const (
	JsonCodec CodecType = 0x1
	ProtoBuf  CodecType = 0x2
)

type Codec interface {
	Encode(v interface{}) ([]byte, error)
	Decode(data []byte, v interface{}) error
}

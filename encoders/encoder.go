package encoders

// DataEncoder defines required functions for encoding/decoding data/files.
type DataEncoder interface {
	GetName() string
	EncodeData(data []byte, config map[string]interface{}) ([]byte, error)
	DecodeData(data []byte, config map[string]interface{}) ([]byte, error)
}

//DataEncoders contains the data encoder implementations
var DataEncoders = map[string]DataEncoder{}
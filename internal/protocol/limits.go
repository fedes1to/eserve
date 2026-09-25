package protocol

// request body caps: a binary is the only big upload, everything else is json
const (
	MaxBinarySize   = 512 << 20
	MaxJSONBodySize = 64 << 10
)

package constant

const (
	RequestMethodGet = iota + 1
	RequestMethodPost
	RequestMethodPut
	RequestMethodDelete
	RequestMethodGrpc
)

const (
	RequestMethodGetString    = "GET"
	RequestMethodPostString   = "POST"
	RequestMethodPutString    = "PUT"
	RequestMethodDeleteString = "DELETE"
	RequestMethodGrpcString   = "GRPC"
)

var RequestMethodMap = map[uint32]string{
	RequestMethodGet:    RequestMethodGetString,
	RequestMethodPost:   RequestMethodPostString,
	RequestMethodPut:    RequestMethodPutString,
	RequestMethodDelete: RequestMethodDeleteString,
	RequestMethodGrpc:   RequestMethodGrpcString,
}

var RequestMethodStringMap = map[string]uint32{
	RequestMethodGetString:    RequestMethodGet,
	RequestMethodPostString:   RequestMethodPost,
	RequestMethodPutString:    RequestMethodPut,
	RequestMethodDeleteString: RequestMethodDelete,
	RequestMethodGrpcString:   RequestMethodGrpc,
}

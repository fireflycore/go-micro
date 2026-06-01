package constant

const (
	// RequestMethodGet 表示 HTTP GET 请求动作的日志枚举值。
	RequestMethodGet = iota + 1
	// RequestMethodPost 表示 HTTP POST 请求动作的日志枚举值。
	RequestMethodPost
	// RequestMethodPut 表示 HTTP PUT 请求动作的日志枚举值。
	RequestMethodPut
	// RequestMethodDelete 表示 HTTP DELETE 请求动作的日志枚举值。
	RequestMethodDelete
	// RequestMethodGrpc 表示 gRPC 请求动作的日志枚举值。
	RequestMethodGrpc
)

const (
	// RequestMethodGetString 表示 HTTP GET 请求动作字符串。
	RequestMethodGetString = "GET"
	// RequestMethodPostString 表示 HTTP POST 请求动作字符串。
	RequestMethodPostString = "POST"
	// RequestMethodPutString 表示 HTTP PUT 请求动作字符串。
	RequestMethodPutString = "PUT"
	// RequestMethodDeleteString 表示 HTTP DELETE 请求动作字符串。
	RequestMethodDeleteString = "DELETE"
	// RequestMethodGrpcString 表示 gRPC 请求动作字符串。
	RequestMethodGrpcString = "GRPC"
)

// RequestMethodMap 提供日志枚举值到协议动作字符串的转换。
var RequestMethodMap = map[uint32]string{
	RequestMethodGet:    RequestMethodGetString,
	RequestMethodPost:   RequestMethodPostString,
	RequestMethodPut:    RequestMethodPutString,
	RequestMethodDelete: RequestMethodDeleteString,
	RequestMethodGrpc:   RequestMethodGrpcString,
}

// RequestMethodStringMap 提供协议动作字符串到日志枚举值的转换。
var RequestMethodStringMap = map[string]uint32{
	RequestMethodGetString:    RequestMethodGet,
	RequestMethodPostString:   RequestMethodPost,
	RequestMethodPutString:    RequestMethodPut,
	RequestMethodDeleteString: RequestMethodDelete,
	RequestMethodGrpcString:   RequestMethodGrpc,
}

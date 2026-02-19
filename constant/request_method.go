package constant

const (
	RequestMethodGet = iota + 1
	RequestMethodPost
	RequestMethodPut
	RequestMethodDelete
	RequestMethodGrpc
)

var RequestMethodValueMap = map[uint32]string{
	RequestMethodGet:    "GET",
	RequestMethodPost:   "POST",
	RequestMethodPut:    "PUT",
	RequestMethodDelete: "DELETE",
	RequestMethodGrpc:   "GRPC",
}

var RequestMethodKeyMap = map[string]uint32{
	"GET":    RequestMethodGet,
	"POST":   RequestMethodPost,
	"PUT":    RequestMethodPut,
	"DELETE": RequestMethodDelete,
	"GRPC":   RequestMethodGrpc,
}

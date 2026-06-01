package constant

const (
	// SubjectTypeAnonymous 表示无 token 但命中公共策略的匿名主体。
	SubjectTypeAnonymous = "anonymous"
	// SubjectTypeUser 表示通过用户 token 还原出的用户主体。
	SubjectTypeUser = "user"
	// SubjectTypeService 表示通过服务 token 还原出的服务主体。
	SubjectTypeService = "service"
)

package varlink

func InterfaceNotFound(ctx Context, name string) error {
	type ReplyParameters struct {
		Name string `json:"interface"`
	}
	return ctx.Reply(&ServerOut{
		Error:      "org.varlink.service.InterfaceNotFound",
		Parameters: ReplyParameters{Name: name},
	})
}

func MethodNotFound(ctx Context, name string) error {
	type ReplyParameters struct {
		Name string `json:"method"`
	}
	return ctx.Reply(&ServerOut{
		Error:      "org.varlink.service.MethodNotFound",
		Parameters: ReplyParameters{Name: name},
	})
}

func MethodNotImplemented(ctx Context, name string) error {
	type ReplyParameters struct {
		Name string `json:"method"`
	}
	return ctx.Reply(&ServerOut{
		Error:      "org.varlink.service.MethodNotImplemented",
		Parameters: ReplyParameters{Name: name},
	})
}

func InvalidParameter(ctx Context, parameter string) error {
	type ReplyParameters struct {
		Parameter string `json:"parameter"`
	}
	return ctx.Reply(&ServerOut{
		Error:      "org.varlink.service.InvalidParameter",
		Parameters: ReplyParameters{Parameter: parameter},
	})
}

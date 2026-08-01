package result

type Envelope struct {
	Success bool           `json:"success"`
	Service string         `json:"service"`
	Tool    string         `json:"tool"`
	Data    any            `json:"data,omitempty"`
	Error   *Error         `json:"error,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

type Error struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	HTTPCode int    `json:"httpStatus,omitempty"`
	Detail   any    `json:"detail,omitempty"`
}

func OK(service, tool string, data any) Envelope {
	return Envelope{Success: true, Service: service, Tool: tool, Data: data}
}

func Fail(service, tool, code, message string) Envelope {
	return Envelope{Success: false, Service: service, Tool: tool, Error: &Error{Code: code, Message: message}}
}

func FailHTTP(service, tool, code, message string, status int) Envelope {
	return FailHTTPDetail(service, tool, code, message, status, nil)
}

func FailHTTPDetail(service, tool, code, message string, status int, detail any) Envelope {
	return Envelope{Success: false, Service: service, Tool: tool, Error: &Error{Code: code, Message: message, HTTPCode: status, Detail: detail}}
}

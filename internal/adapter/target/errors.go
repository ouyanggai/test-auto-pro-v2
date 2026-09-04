package target

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorLoginRejected    ErrorKind = "login_rejected"
	ErrorSessionExpired   ErrorKind = "session_expired"
	ErrorResponseInvalid  ErrorKind = "response_invalid"
	ErrorUnavailable      ErrorKind = "unavailable"
	ErrorTimeout          ErrorKind = "timeout"
	ErrorPermissionDenied ErrorKind = "permission_denied"
)

// Error 对外只暴露稳定分类，不携带目标平台原始报文。
// Transport 是传输层失败阶段的独立事实（见 transport.go），仅在传输层失败或完整响应后填充，
// 供执行器构造判定包输入；未经过传输的配置类错误保持空值。
type Error struct {
	Kind       ErrorKind
	HTTPStatus int
	Transport  TransportPhase
	Cause      error
}

// Error 返回不含目标原始响应的稳定错误分类。
func (e *Error) Error() string {
	if e == nil {
		return "目标平台请求失败"
	}
	switch e.Kind {
	case ErrorLoginRejected:
		return "目标平台拒绝登录"
	case ErrorSessionExpired:
		return "目标平台会话已失效"
	case ErrorResponseInvalid:
		return "目标平台响应格式异常"
	case ErrorTimeout:
		return "目标平台请求超时"
	case ErrorPermissionDenied:
		return "目标平台拒绝访问"
	default:
		return "目标平台暂时不可用"
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewError(kind ErrorKind, cause error) error {
	return &Error{Kind: kind, Cause: cause}
}

func errorWithStatus(kind ErrorKind, status int, cause error) error {
	return &Error{Kind: kind, HTTPStatus: status, Cause: cause}
}

func IsKind(err error, kind ErrorKind) bool {
	var targetErr *Error
	return errors.As(err, &targetErr) && targetErr.Kind == kind
}

func asError(err error) *Error {
	var targetErr *Error
	if errors.As(err, &targetErr) {
		return targetErr
	}
	return nil
}

func invalidResponse(reason string) error {
	return NewError(ErrorResponseInvalid, fmt.Errorf("invalid target response: %s", reason))
}

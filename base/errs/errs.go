package errs

import "fmt"

// ErrCode 错误码
type ErrCode = int32

// Error 通用错误接口
type Error interface {
	error

	// Code 返回错误码
	Code() ErrCode

	// Msg 返回错误信息
	Msg() string
}

// errCodeMsg 错误码与错误信息
type errCodeMsg struct {
	code ErrCode // 错误码
	msg  string  // 错误信息
}

// Error 实现 error 接口
func (e *errCodeMsg) Error() string {
	return fmt.Sprintf("%d: %s", e.code, e.msg)
}

// Code 返回错误码
func (e *errCodeMsg) Code() ErrCode {
	return e.code
}

// Msg 返回错误信息
func (e *errCodeMsg) Msg() string {
	return e.msg
}

// errCodeErr 错误码与错误信息
type errCodeErr struct {
	code ErrCode // 错误码
	err  error   // 错误
}

// Error 实现 error 接口
func (e *errCodeErr) Error() string {
	return fmt.Sprintf("%d: %v", e.code, e.err)
}

// Code 返回错误码
func (e *errCodeErr) Code() ErrCode {
	return e.code
}

// Msg 返回错误信息
func (e *errCodeErr) Msg() string {
	return e.err.Error()
}

// WithErrCodeMsg 创建包含错误码及错误信息的 Error
func WithErrCodeMsg(code ErrCode, msg string) Error {
	return &errCodeMsg{
		code: code,
		msg:  msg,
	}
}

// WithErrCodeMsgf 创建包含错误码及格式化错误信息的 Error
func WithErrCodeMsgf(code ErrCode, format string, a ...any) Error {
	return &errCodeMsg{
		code: code,
		msg:  fmt.Sprintf(format, a...),
	}
}

// WithErrCodeErr 创建包含错误码及错误的 Error
func WithErrCodeErr(code ErrCode, err error) Error {
	return &errCodeErr{
		code: code,
		err:  err,
	}
}

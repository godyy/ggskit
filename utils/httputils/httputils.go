package httputils

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	pkgerrors "github.com/pkg/errors"
)

// maxTimeout 最大超时
const maxTimeout = 10 * time.Second

// client 客户端实例
var client = &http.Client{
	Timeout: maxTimeout,
}

// Request 用于代理执行http请求.
type Request struct {
	Method      string          // post/get/head...
	Url         string          // url地址.
	ContentType string          // 对于特殊方法有效, 比如 post.
	Body        []byte          // request body, 格式对应 ContentType.
	Ctx         context.Context // 可选, 用于控制请求的生命周期

	requestOptions   []RequestOption   // 请求选项, 在请求发送之前设置
	responseCheckers []ResponseChecker // 响应检查器, 在响应返回之后检查

}

// AddRequestOptions 添加请求选项.
func (r *Request) AddRequestOptions(options ...RequestOption) *Request {
	r.requestOptions = append(r.requestOptions, options...)
	return r
}

// AddResponseCheckers 添加响应检查器.
func (r *Request) AddResponseCheckers(checkers ...ResponseChecker) *Request {
	r.responseCheckers = append(r.responseCheckers, checkers...)
	return r
}

// Do 执行请求.
func (r *Request) Do() (*http.Response, error) {
	// 请求体
	var bodyReader io.Reader
	if r.Body != nil {
		bodyReader = bytes.NewReader(r.Body)
	}

	// 使用调用方提供的上下文；未提供则用 Background
	ctx := r.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// 创建http.Request
	httpReq, err := http.NewRequestWithContext(ctx, r.Method, r.Url, bodyReader)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "new request")
	}

	// Content-Type
	if r.ContentType != "" {
		httpReq.Header.Set("content-type", r.ContentType)
	}

	// 请求选项
	for _, v := range r.requestOptions {
		v(httpReq)
	}

	// 调用请求
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "do request")
	}

	// 响应检查
	for _, v := range r.responseCheckers {
		if err := v(httpResp); err != nil {
			if httpResp.Body != nil {
				httpResp.Body.Close()
			}
			return nil, err
		}
	}

	return httpResp, nil
}

// PostJson 执行一个post请求, 请求和响应body均为json格式.
// 使用默认最大超时.
func PostJson(url string, req, resp any, headerFunc func(http.Header)) error {
	return PostJsonWithContext(context.Background(), url, req, resp, headerFunc)
}

// PostJsonWithContext 执行一个post请求, 请求和响应body均为json格式.
// ctx 控制请求的生命周期.
func PostJsonWithContext(ctc context.Context, url string, req, resp any, headerFunc func(http.Header)) error {
	// 编码请求reqBody.
	reqBody, err := json.Marshal(req)
	if err != nil {
		return pkgerrors.WithMessage(err, "marshal req")
	}

	// 构造请求
	r := Request{
		Method:      http.MethodPost,
		Url:         url,
		ContentType: "application/json",
		Body:        reqBody,
	}
	if headerFunc != nil {
		r.AddRequestOptions(WithHeaderFunc(headerFunc))
	}
	r.AddResponseCheckers(
		WithCheckHead("content-type", "application/json", true),
	)

	// 执行请求
	httpResp, err := r.Do()
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	// 解码响应body.
	if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
		return pkgerrors.WithMessage(err, "decode resp")
	}

	return nil
}

// GetJson 执行get请求, 请求参数为url中的查询参数, 响应数据为json格式.
// 使用默认最大超时.
func GetJson(url string, resp any, headerFunc func(http.Header)) error {
	return GetJsonWithContext(context.Background(), url, resp, headerFunc)
}

// GetJsonWithContext 执行get请求, 请求参数为url中的查询参数, 响应数据为json格式.
// ctx 控制请求的生命周期.
func GetJsonWithContext(ctx context.Context, url string, resp any, headerFunc func(http.Header)) error {
	// 构造请求
	r := Request{
		Method:      http.MethodGet,
		Url:         url,
		ContentType: "application/json",
		Ctx:         ctx,
	}
	if headerFunc != nil {
		r.AddRequestOptions(WithHeaderFunc(headerFunc))
	}
	r.AddResponseCheckers(
		WithCheckHead("content-type", "application/json", true),
	)

	// 执行请求
	httpResp, err := r.Do()
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	// 解码响应body.
	if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
		return pkgerrors.WithMessage(err, "decode resp")
	}

	return nil
}

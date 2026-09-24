package target

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptrace"
	"strings"
)

// uploadFileEndpoint 是目标文件服务的单文件上传接口（参考 rsh-flow-components api/index.js: uploadFile）。
// 它属于文件服务，不是动作目录里的业务流程写端点，因此不进入 F-012 的「业务零写」白名单/静态断言。
const uploadFileEndpoint = "/web/file/api/file/uploadFile"

// UploadedFile 是目标文件服务上传成功后返回的文件引用（已核实公开字段）。
// FormMaking 上传组件回显与提交都依赖这些引用，本地静态文件没有它们目标不认。
type UploadedFile struct {
	ID                string
	FileID            string
	FileName          string
	OriginFileName    string
	FileURL           string
	AbsolutelyFileURL string
	FileType          string
}

// UploadFile 以 multipart 向目标文件服务上传一个文件，返回目标可识别的文件引用。
// 边界与约束：
//   - 目标上传接口有时较慢：等待上限完全由传入的 ctx 决定，本方法不设无限等待，也不内建重试，
//     由调用方按「一次性上传+缓存复用」控制频率，避免慢接口把配置流程整体拖住。
//   - 只发一次上传请求；响应中断按不确定失败上报，绝不自动重发（避免产生重复文件）。
//   - 该接口是文件服务上传，非业务流程写；沿用统一传输探针与错误分档，class 记为 read。
func (c *Client) UploadFile(ctx context.Context, sid, filename string, content []byte) (UploadedFile, error) {
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return UploadedFile{}, invalidResponse("missing sid for upload")
	}
	// 组装 multipart：目标接口取 multipartFile 字段的文件二进制，并额外携带 sid 表单字段（参考上传组件实现）。
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile("multipartFile", filename)
	if err != nil {
		return UploadedFile{}, invalidResponse("cannot build upload body")
	}
	if _, err := part.Write(content); err != nil {
		return UploadedFile{}, invalidResponse("cannot write upload body")
	}
	if err := writer.WriteField("sid", sid); err != nil {
		return UploadedFile{}, invalidResponse("cannot write upload field")
	}
	if err := writer.Close(); err != nil {
		return UploadedFile{}, invalidResponse("cannot finalize upload body")
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + uploadFileEndpoint
	endpoint.RawPath = ""
	query := endpoint.Query()
	if c.config.PlatformCode != "" {
		query.Set("platformCode", c.config.PlatformCode)
	}
	query.Set("sid", sid)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), &buffer)
	if err != nil {
		return UploadedFile{}, NewError(ErrorUnavailable, err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("sid", sid)
	req.Header.Set("origin", strings.TrimRight(c.baseURL.String(), "/"))
	req.Header.Set("Referer", strings.TrimRight(c.baseURL.String(), "/")+"/")
	// 传输阶段分档是安全判定前提：连接未完成（确定失败、无副作用）与响应丢失（不确定）结论相反，
	// 必须在请求发出前挂上 httptrace 才能拿到判据。
	probe := &transportProbe{}
	req = req.WithContext(withRequestMetadata(httptrace.WithClientTrace(req.Context(), probe.trace()), "read", ""))
	response, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return UploadedFile{}, err
		}
		phase := probe.classify(err)
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return UploadedFile{}, &Error{Kind: ErrorTimeout, Cause: err, Transport: phase}
		}
		return UploadedFile{}, &Error{Kind: ErrorUnavailable, Cause: err, Transport: phase}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return UploadedFile{}, &Error{Kind: ErrorUnavailable, Cause: err, Transport: TransportInterrupted}
	}
	if len(data) > maxResponseBytes {
		return UploadedFile{}, &Error{Kind: ErrorResponseInvalid, Transport: TransportResponded, Cause: errors.New("response too large")}
	}
	if response.StatusCode == http.StatusUnauthorized {
		return UploadedFile{}, &Error{Kind: ErrorSessionExpired, HTTPStatus: response.StatusCode, Transport: TransportResponded, Cause: httpResponseDetail(data)}
	}
	if response.StatusCode == http.StatusForbidden {
		return UploadedFile{}, &Error{Kind: ErrorPermissionDenied, HTTPStatus: response.StatusCode, Transport: TransportResponded, Cause: httpResponseDetail(data)}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return UploadedFile{}, &Error{Kind: ErrorUnavailable, HTTPStatus: response.StatusCode, Transport: TransportResponded, Cause: httpResponseDetail(data)}
	}
	var result envelope
	if err := json.Unmarshal(data, &result); err != nil {
		return UploadedFile{}, &Error{Kind: ErrorResponseInvalid, Transport: TransportResponded, Cause: errors.New("invalid json")}
	}
	if responseSessionExpired(&result) {
		return UploadedFile{}, &Error{Kind: ErrorSessionExpired, Transport: TransportResponded, Cause: responseDetail(&result)}
	}
	if !responseSucceeded(&result) {
		return UploadedFile{}, responseError(&result)
	}
	var raw struct {
		ID                string `json:"id"`
		FileID            string `json:"fileId"`
		FileName          string `json:"fileName"`
		OriginFileName    string `json:"originFileName"`
		FileURL           string `json:"fileUrl"`
		AbsolutelyFileURL string `json:"absolutelyFileUrl"`
		FileType          string `json:"fileType"`
	}
	if len(result.Data) == 0 || string(result.Data) == "null" {
		return UploadedFile{}, invalidResponse("empty upload result")
	}
	if err := json.Unmarshal(result.Data, &raw); err != nil {
		return UploadedFile{}, invalidResponse("invalid upload result")
	}
	uploaded := UploadedFile{
		ID: strings.TrimSpace(raw.ID), FileID: strings.TrimSpace(raw.FileID),
		FileName: strings.TrimSpace(raw.FileName), OriginFileName: strings.TrimSpace(raw.OriginFileName),
		FileURL: strings.TrimSpace(raw.FileURL), AbsolutelyFileURL: strings.TrimSpace(raw.AbsolutelyFileURL),
		FileType: strings.TrimSpace(raw.FileType),
	}
	// 目标必须至少返回一个可用文件标识，否则回显与提交都无法引用该附件，按无效响应处理。
	if uploaded.ID == "" && uploaded.FileID == "" {
		return UploadedFile{}, invalidResponse("upload result missing file id")
	}
	return uploaded, nil
}

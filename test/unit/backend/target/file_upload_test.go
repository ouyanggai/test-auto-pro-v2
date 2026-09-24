// 目标文件服务上传契约：一键配置补齐必填附件时以 multipart 上传占位文件，
// 请求必须带 multipartFile 二进制、sid 表单字段、sid/platformCode 查询参数，
// 并把目标返回的文件引用解析为可回填的 UploadedFile；缺少文件标识按无效响应拒绝。
package target_test

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestUploadFileSendsMultipartAndParsesRef 验证上传请求的 multipart 契约与响应解析。
func TestUploadFileSendsMultipartAndParsesRef(t *testing.T) {
	var gotFilename, gotFieldSID, gotQuerySID, gotPlatform string
	var gotContent []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuerySID = r.URL.Query().Get("sid")
		gotPlatform = r.URL.Query().Get("platformCode")
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			t.Errorf("上传请求必须是 multipart：%s", r.Header.Get("Content-Type"))
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err != nil {
				break
			}
			switch part.FormName() {
			case "multipartFile":
				gotFilename = part.FileName()
				gotContent, _ = io.ReadAll(part)
			case "sid":
				body, _ := io.ReadAll(part)
				gotFieldSID = string(body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"isSuccess": true,
			"data": map[string]any{
				"id": "REL-1", "fileId": "FILE-9", "fileUrl": "/f/9",
				"absolutelyFileUrl": "http://t/f/9", "originFileName": "占位.txt", "fileType": "ordinaryFile",
			},
		})
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, PlatformCode: "200001", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	ref, err := client.UploadFile(context.Background(), "sid-abc", "占位.txt", []byte("hello"))
	if err != nil {
		t.Fatalf("上传失败：%v", err)
	}
	if gotFilename != "占位.txt" || string(gotContent) != "hello" {
		t.Fatalf("multipartFile 文件名或内容不对：name=%q content=%q", gotFilename, string(gotContent))
	}
	if gotFieldSID != "sid-abc" || gotQuerySID != "sid-abc" {
		t.Fatalf("sid 未同时进入表单字段与查询参数：field=%q query=%q", gotFieldSID, gotQuerySID)
	}
	if gotPlatform != "200001" {
		t.Fatalf("platformCode 查询参数缺失：%q", gotPlatform)
	}
	if ref.FileID != "FILE-9" || ref.ID != "REL-1" || ref.AbsolutelyFileURL != "http://t/f/9" || ref.OriginFileName != "占位.txt" {
		t.Fatalf("上传返回引用解析不正确：%+v", ref)
	}
}

// TestUploadFileRejectsResultWithoutFileID 验证目标返回成功但缺少文件标识时按无效响应拒绝，
// 避免把无法引用的空附件当作补齐成功。
func TestUploadFileRejectsResultWithoutFileID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"isSuccess": true,
			"data":      map[string]any{"fileUrl": "/f/none"},
		})
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	if _, err := client.UploadFile(context.Background(), "sid-abc", "占位.txt", []byte("hello")); err == nil {
		t.Fatal("缺少文件标识的上传结果必须报错")
	}
}

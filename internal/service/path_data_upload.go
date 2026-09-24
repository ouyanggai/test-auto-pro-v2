package service

import (
	"context"
	_ "embed"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// placeholderAttachment 是随二进制内嵌的占位附件内容；配置期需要补齐必填附件时上传它。
//
//go:embed assets/placeholder-attachment.txt
var placeholderAttachment []byte

// placeholderAttachmentName 是占位附件在目标文件服务中的文件名。
const placeholderAttachmentName = "流程自动化测试平台占位附件.txt"

// placeholderUploadTimeout 是单次占位附件上传的等待上限：目标上传接口有时较慢，
// 给足够宽裕的墙钟预算，但绝不无限等待，超时按上传失败阻断，不让配置流程被慢接口拖死。
const placeholderUploadTimeout = 180 * time.Second

// placeholderUploadRefs 按账号缓存占位附件的上传结果，实现「一次性真实上传、跨字段与路径复用」。
// 只在进程内缓存：慢上传接口每个账号最多被调用一次，进程重启后按需再上传一次。
var placeholderUploadRefs sync.Map // account(string) -> target.UploadedFile

// fileUploader 是配置期补齐所需的最小上传能力；目标 Client 实现它。
type fileUploader interface {
	UploadFile(ctx context.Context, sid, filename string, content []byte) (target.UploadedFile, error)
}

// sessionProvider 提供计划账号的目标运行时会话（含 SID），供上传取得鉴权。
type sessionProvider interface {
	FormRuntimeSession(context.Context, string) (target.FormRuntimeSession, error)
}

// newPlaceholderUploadFill 构造「一次性真实上传占位附件并跨字段/路径复用」的上传回填闭包。
// targetReader 不具备上传或会话能力时返回 nil：调用方据此对必填附件落阻断（持久化路径）
// 或给非阻断待上传提示（只读预览）。闭包内 once 确保同一次调用最多穿透一次缓存查询。
func newPlaceholderUploadFill(ctx context.Context, targetReader any, account string) func(target.FormFieldDetail) (any, bool) {
	uploader, canUpload := targetReader.(fileUploader)
	sessions, canSession := targetReader.(sessionProvider)
	if !canUpload || !canSession {
		return nil
	}
	var (
		once sync.Once
		ref  target.UploadedFile
		err  error
	)
	return func(field target.FormFieldDetail) (any, bool) {
		once.Do(func() {
			ref, err = uploadPlaceholderOnce(ctx, uploader, sessions, account)
		})
		if err != nil {
			return nil, false
		}
		return uploadValueForField(ref), true
	}
}

// uploadPlaceholderOnce 优先命中账号级缓存，未命中时以受限超时上传一次占位附件并写回缓存。
// 上传只发一次、不自动重发：慢或中断都按失败上报，绝不产生重复文件。
func uploadPlaceholderOnce(ctx context.Context, uploader fileUploader, sessions sessionProvider, account string) (target.UploadedFile, error) {
	if cached, ok := placeholderUploadRefs.Load(account); ok {
		return cached.(target.UploadedFile), nil
	}
	var active target.FormRuntimeSession
	if err := retryTransientTargetRead(ctx, 3, func(ctx context.Context) error {
		session, sessionErr := sessions.FormRuntimeSession(ctx, account)
		if sessionErr != nil {
			return sessionErr
		}
		active = session
		return nil
	}); err != nil {
		return target.UploadedFile{}, err
	}
	uploadCtx, cancel := context.WithTimeout(ctx, placeholderUploadTimeout)
	defer cancel()
	ref, err := uploader.UploadFile(uploadCtx, active.SID, placeholderAttachmentName, placeholderAttachment)
	if err != nil {
		return target.UploadedFile{}, err
	}
	placeholderUploadRefs.Store(account, ref)
	return ref, nil
}

// uploadValueForField 把目标上传返回的文件引用组装为 FormMaking 上传组件可识别的回显值。
// 目标各上传组件回显键各不相同（name/originFileName、id/fileId、fileUrl/absolutelyFileUrl/url），
// 这里提供已核实键的超集单元素数组，组件各取所需；具体组件形状最终由用户在浏览器人工核对确认。
func uploadValueForField(ref target.UploadedFile) any {
	name := firstNonEmptyString(ref.OriginFileName, ref.FileName, placeholderAttachmentName)
	url := firstNonEmptyString(ref.AbsolutelyFileURL, ref.FileURL)
	entry := map[string]any{
		"name":              name,
		"originFileName":    name,
		"id":                firstNonEmptyString(ref.ID, ref.FileID),
		"fileId":            firstNonEmptyString(ref.FileID, ref.ID),
		"fileUrl":           ref.FileURL,
		"absolutelyFileUrl": url,
		"url":               url,
		"fileType":          ref.FileType,
		"status":            "uploaded",
		"percentage":        100,
	}
	return []any{entry}
}

// UploadValueForFieldForTest 暴露上传回显值组装，供 test 目录锁定超集形状与关键引用键。
func UploadValueForFieldForTest(ref target.UploadedFile) any {
	return uploadValueForField(ref)
}

// ResetPlaceholderUploadCacheForTest 清空账号级上传缓存，保证定向测试之间互不串扰。
func ResetPlaceholderUploadCacheForTest() {
	placeholderUploadRefs.Range(func(key, _ any) bool {
		placeholderUploadRefs.Delete(key)
		return true
	})
}

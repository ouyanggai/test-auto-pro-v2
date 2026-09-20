package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"test-auto-pro-v2/internal/model"
)

// AddSignPersonExclusionService 定义排除名单服务接口
type AddSignPersonExclusionService interface {
	Add(r *http.Request, item *model.AddSignPersonExclusionItem) error
	Remove(r *http.Request, personID string) error
	List(r *http.Request, nameFilter string) ([]model.AddSignPersonExclusionItem, error)
}

// AddSignCandidateDTO 是按姓名检索目标人员目录的返回项，供前端点选加入排除名单。
type AddSignCandidateDTO struct {
	PersonID   string `json:"personId"`
	PersonName string `json:"personName"`
	Phone      string `json:"phone"`
	Enabled    bool   `json:"enabled"`
}

// AutoExcludedPersonDTO 是系统默认排除的人员及原因；reason: disabled | no_platform_role。
type AutoExcludedPersonDTO struct {
	PersonID   string `json:"personId"`
	PersonName string `json:"personName"`
	Phone      string `json:"phone"`
	Reason     string `json:"reason"`
}

// AddSignCandidateSearcher 按姓名检索目标人员目录、并列出系统自动排除人员；
// 未配置目标只读库时为 nil，对应端点降级。
type AddSignCandidateSearcher interface {
	SearchCandidates(r *http.Request, name string) ([]AddSignCandidateDTO, error)
	ListAutoExcluded(r *http.Request, name string) ([]AutoExcludedPersonDTO, error)
}

// exclusionListItemDTO 是人员排除列表的统一项：手动名单与系统自动排除合并展示。
// source: manual（手动，可删除）| auto（系统默认排除，只读）。
// autoReason 仅 source=auto 时有值：disabled（账号已停用）| no_platform_role（未分配当前平台权限）。
type exclusionListItemDTO struct {
	PersonID    string `json:"personId"`
	PersonName  string `json:"personName"`
	PersonPhone string `json:"personPhone"`
	Reason      string `json:"reason"`
	Source      string `json:"source"`
	AutoReason  string `json:"autoReason,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// registerAddSignPersonExclusionRoutes 注册排除名单相关路由；searcher 为空时不注册人员检索端点。
func registerAddSignPersonExclusionRoutes(mux *http.ServeMux, service AddSignPersonExclusionService, searcher AddSignCandidateSearcher) {
	mux.HandleFunc("GET /api/settings/add-sign-exclusion", handleListAddSignExclusion(service, searcher))
	mux.HandleFunc("POST /api/settings/add-sign-exclusion", handleAddSignExclusion(service))
	mux.HandleFunc("DELETE /api/settings/add-sign-exclusion/{personId}", handleRemoveAddSignExclusion(service))
	if searcher != nil {
		mux.HandleFunc("GET /api/settings/add-sign-exclusion/candidates", handleSearchAddSignCandidates(searcher))
	}
}

// handleSearchAddSignCandidates 按姓名检索目标人员目录，供前端点选加入排除名单。
func handleSearchAddSignCandidates(searcher AddSignCandidateSearcher) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		name := strings.TrimSpace(request.URL.Query().Get("name"))
		if name == "" {
			writeSuccess(response, map[string]interface{}{"items": []AddSignCandidateDTO{}, "total": 0})
			return
		}
		items, err := searcher.SearchCandidates(request, name)
		if err != nil {
			writeFailure(response, http.StatusInternalServerError, "SEARCH_CANDIDATE_FAILED", "检索目标人员失败", true)
			return
		}
		if items == nil {
			items = []AddSignCandidateDTO{}
		}
		writeSuccess(response, map[string]interface{}{"items": items, "total": len(items)})
	}
}

// handleListAddSignExclusion 获取人员排除列表：手动名单 + 系统自动排除（停用/无平台角色）合并展示。
func handleListAddSignExclusion(service AddSignPersonExclusionService, searcher AddSignCandidateSearcher) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		nameFilter := strings.TrimSpace(request.URL.Query().Get("name"))

		manual, err := service.List(request, nameFilter)
		if err != nil {
			writeFailure(response, http.StatusInternalServerError, "QUERY_EXCLUSION_FAILED", "查询排除名单失败", true)
			return
		}
		items := make([]exclusionListItemDTO, 0, len(manual))
		manualIDs := make(map[string]bool, len(manual))
		for _, item := range manual {
			manualIDs[item.PersonID] = true
			items = append(items, exclusionListItemDTO{
				PersonID: item.PersonID, PersonName: item.PersonName, PersonPhone: item.PersonPhone,
				Reason: item.Reason, Source: "manual", UpdatedAt: item.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		manualCount := len(items)

		// 系统自动排除：接入目标目录时一并列出；已在手动名单里的不重复展示。目录读取失败不影响手动名单返回。
		autoCount := 0
		if searcher != nil {
			if autos, autoErr := searcher.ListAutoExcluded(request, nameFilter); autoErr == nil {
				for _, auto := range autos {
					if manualIDs[auto.PersonID] {
						continue
					}
					items = append(items, exclusionListItemDTO{
						PersonID: auto.PersonID, PersonName: auto.PersonName, PersonPhone: auto.Phone,
						Reason: autoReasonText(auto.Reason), Source: "auto", AutoReason: auto.Reason,
					})
					autoCount++
				}
			}
		}

		writeSuccess(response, map[string]interface{}{
			"items":       items,
			"total":       len(items),
			"manualTotal": manualCount,
			"autoTotal":   autoCount,
		})
	}
}

// autoReasonText 把自动排除原因编码转成中文说明。
func autoReasonText(reason string) string {
	switch reason {
	case "disabled":
		return "账号已停用"
	case "no_platform_role":
		return "未分配当前平台权限"
	default:
		return "系统自动排除"
	}
}

// handleAddSignExclusion 添加或更新排除名单
func handleAddSignExclusion(service AddSignPersonExclusionService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.ContentLength == 0 {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "请提供排除人员信息", false)
			return
		}

		var req model.AddSignPersonExclusionRequest
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_JSON", "请求体格式错误", false)
			return
		}

		req.PersonID = strings.TrimSpace(req.PersonID)
		req.PersonName = strings.TrimSpace(req.PersonName)
		if req.PersonID == "" || req.PersonName == "" {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "personId 和 personName 不能为空", false)
			return
		}

		item := &model.AddSignPersonExclusionItem{
			PersonID:    req.PersonID,
			PersonName:  req.PersonName,
			PersonPhone: strings.TrimSpace(req.PersonPhone),
			Reason:      strings.TrimSpace(req.Reason),
		}

		if err := service.Add(request, item); err != nil {
			if strings.Contains(err.Error(), "不能为空") {
				writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), false)
			} else {
				writeFailure(response, http.StatusInternalServerError, "ADD_EXCLUSION_FAILED", "添加排除名单失败", true)
			}
			return
		}

		writeJSON(response, http.StatusCreated, apiSuccess{Success: true, Data: item})
	}
}

// handleRemoveAddSignExclusion 删除排除名单项
func handleRemoveAddSignExclusion(service AddSignPersonExclusionService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		personID := strings.TrimSpace(request.PathValue("personId"))
		if personID == "" {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "personId 不能为空", false)
			return
		}

		if err := service.Remove(request, personID); err != nil {
			if errors.Is(err, errors.New("未找到")) || strings.Contains(err.Error(), "未找到") {
				writeFailure(response, http.StatusNotFound, "EXCLUSION_NOT_FOUND", "排除名单中不存在该人员", false)
			} else {
				writeFailure(response, http.StatusInternalServerError, "REMOVE_EXCLUSION_FAILED", "删除排除名单失败", true)
			}
			return
		}

		writeSuccess(response, map[string]string{"message": "删除成功"})
	}
}

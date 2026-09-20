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

// AddSignCandidateSearcher 按姓名检索目标人员目录；未配置目标只读库时为 nil，对应端点返回不可用。
type AddSignCandidateSearcher interface {
	SearchCandidates(r *http.Request, name string) ([]AddSignCandidateDTO, error)
}

// registerAddSignPersonExclusionRoutes 注册排除名单相关路由；searcher 为空时不注册人员检索端点。
func registerAddSignPersonExclusionRoutes(mux *http.ServeMux, service AddSignPersonExclusionService, searcher AddSignCandidateSearcher) {
	mux.HandleFunc("GET /api/settings/add-sign-exclusion", handleListAddSignExclusion(service))
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

// handleListAddSignExclusion 获取排除名单列表
func handleListAddSignExclusion(service AddSignPersonExclusionService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		// 从查询参数读取名字过滤条件
		nameFilter := strings.TrimSpace(request.URL.Query().Get("name"))

		items, err := service.List(request, nameFilter)
		if err != nil {
			writeFailure(response, http.StatusInternalServerError, "QUERY_EXCLUSION_FAILED", "查询排除名单失败", true)
			return
		}

		if items == nil {
			items = []model.AddSignPersonExclusionItem{}
		}

		writeSuccess(response, map[string]interface{}{
			"items": items,
			"total": len(items),
		})
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

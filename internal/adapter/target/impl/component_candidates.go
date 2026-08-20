package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// ComponentCandidateAdapter 实现目标平台的组件候选提供者。
type ComponentCandidateAdapter struct {
	client *Client
}

// NewComponentCandidateAdapter 创建组件候选适配器。
func NewComponentCandidateAdapter(client *Client) *ComponentCandidateAdapter {
	return &ComponentCandidateAdapter{client: client}
}

// GetMaterialCandidates 获取材料选择组件的候选项。
func (a *ComponentCandidateAdapter) GetMaterialCandidates(ctx context.Context, account, flowCode, direction string) ([]target.MaterialCandidate, error) {
	session, err := a.client.session(ctx, account)
	if err != nil {
		return nil, err
	}

	endpoint := "/api/material/list"
	if direction == "in" {
		endpoint = "/api/material/inbound/list"
	} else if direction == "out" {
		endpoint = "/api/material/outbound/list"
	}

	type materialResponse struct {
		IsSuccess bool   `json:"isSuccess"`
		Message   string `json:"message"`
		Data      []struct {
			ID            string  `json:"id"`
			Name          string  `json:"name"`
			Code          string  `json:"code"`
			Specification string  `json:"specification"`
			Unit          string  `json:"unit"`
			Price         float64 `json:"price"`
			Category      string  `json:"category"`
		} `json:"data"`
	}

	req, err := a.buildRequest(ctx, session, "POST", endpoint, map[string]any{
		"page":     1,
		"pageSize": 100,
		"status":   "active",
	})
	if err != nil {
		return nil, err
	}

	var resp materialResponse
	if err := a.doRequest(req, &resp); err != nil {
		return nil, err
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("获取材料候选失败: %s", resp.Message)
	}

	candidates := make([]target.MaterialCandidate, 0, len(resp.Data))
	for _, item := range resp.Data {
		candidates = append(candidates, target.MaterialCandidate{
			ID:            item.ID,
			Name:          item.Name,
			Code:          item.Code,
			Specification: item.Specification,
			Unit:          item.Unit,
			Price:         item.Price,
			Category:      item.Category,
		})
	}

	return candidates, nil
}

// GetProjectCandidates 获取项目选择组件的候选项。
func (a *ComponentCandidateAdapter) GetProjectCandidates(ctx context.Context, account, flowCode string) ([]target.ProjectCandidate, error) {
	session, err := a.client.session(ctx, account)
	if err != nil {
		return nil, err
	}

	type projectResponse struct {
		IsSuccess bool   `json:"isSuccess"`
		Message   string `json:"message"`
		Data      []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Code        string `json:"code"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			Manager     string `json:"manager"`
			CompanyID   string `json:"companyId"`
			CompanyName string `json:"companyName"`
		} `json:"data"`
	}

	req, err := a.buildRequest(ctx, session, "POST", "/api/project/list", map[string]any{
		"page":     1,
		"pageSize": 100,
		"status":   "active",
	})
	if err != nil {
		return nil, err
	}

	var resp projectResponse
	if err := a.doRequest(req, &resp); err != nil {
		return nil, err
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("获取项目候选失败: %s", resp.Message)
	}

	candidates := make([]target.ProjectCandidate, 0, len(resp.Data))
	for _, item := range resp.Data {
		candidates = append(candidates, target.ProjectCandidate{
			ID:          item.ID,
			Name:        item.Name,
			Code:        item.Code,
			Type:        item.Type,
			Status:      item.Status,
			Manager:     item.Manager,
			CompanyID:   item.CompanyID,
			CompanyName: item.CompanyName,
		})
	}

	return candidates, nil
}

// GetOrderCandidates 获取订单选择组件的候选项。
func (a *ComponentCandidateAdapter) GetOrderCandidates(ctx context.Context, account, flowCode string) ([]target.OrderCandidate, error) {
	session, err := a.client.session(ctx, account)
	if err != nil {
		return nil, err
	}

	type orderResponse struct {
		IsSuccess bool   `json:"isSuccess"`
		Message   string `json:"message"`
		Data      []struct {
			ID          string  `json:"id"`
			OrderNo     string  `json:"orderNo"`
			Type        string  `json:"type"`
			Amount      float64 `json:"amount"`
			Status      string  `json:"status"`
			CreateDate  string  `json:"createDate"`
			Description string  `json:"description"`
		} `json:"data"`
	}

	req, err := a.buildRequest(ctx, session, "POST", "/api/order/list", map[string]any{
		"page":     1,
		"pageSize": 100,
		"status":   "active",
	})
	if err != nil {
		return nil, err
	}

	var resp orderResponse
	if err := a.doRequest(req, &resp); err != nil {
		return nil, err
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("获取订单候选失败: %s", resp.Message)
	}

	candidates := make([]target.OrderCandidate, 0, len(resp.Data))
	for _, item := range resp.Data {
		candidates = append(candidates, target.OrderCandidate{
			ID:          item.ID,
			OrderNo:     item.OrderNo,
			Type:        item.Type,
			Amount:      item.Amount,
			Status:      item.Status,
			CreateDate:  item.CreateDate,
			Description: item.Description,
		})
	}

	return candidates, nil
}

// GetFlowListCandidates 获取流程列表选择组件的候选项。
func (a *ComponentCandidateAdapter) GetFlowListCandidates(ctx context.Context, account, flowCode string) ([]target.FlowListCandidate, error) {
	session, err := a.client.session(ctx, account)
	if err != nil {
		return nil, err
	}

	type flowListResponse struct {
		IsSuccess bool   `json:"isSuccess"`
		Message   string `json:"message"`
		Data      []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Type        string `json:"type"`
			DeptID      string `json:"deptId"`
			DeptName    string `json:"deptName"`
			CompanyID   string `json:"companyId"`
			CompanyName string `json:"companyName"`
		} `json:"data"`
	}

	req, err := a.buildRequest(ctx, session, "POST", "/api/flow/list", map[string]any{
		"page":     1,
		"pageSize": 100,
	})
	if err != nil {
		return nil, err
	}

	var resp flowListResponse
	if err := a.doRequest(req, &resp); err != nil {
		return nil, err
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("获取流程列表候选失败: %s", resp.Message)
	}

	candidates := make([]target.FlowListCandidate, 0, len(resp.Data))
	for _, item := range resp.Data {
		candidates = append(candidates, target.FlowListCandidate{
			ID:          item.ID,
			Name:        item.Name,
			Type:        item.Type,
			DeptID:      item.DeptID,
			DeptName:    item.DeptName,
			CompanyID:   item.CompanyID,
			CompanyName: item.CompanyName,
		})
	}

	return candidates, nil
}

// GetExpenseBudgetTypes 获取费用预算类型候选项。
func (a *ComponentCandidateAdapter) GetExpenseBudgetTypes(ctx context.Context, account string) ([]target.ExpenseBudgetType, error) {
	session, err := a.client.session(ctx, account)
	if err != nil {
		return nil, err
	}

	type budgetResponse struct {
		IsSuccess bool   `json:"isSuccess"`
		Message   string `json:"message"`
		Data      []struct {
			ID          string  `json:"id"`
			Name        string  `json:"name"`
			Code        string  `json:"code"`
			Budget      float64 `json:"budget"`
			Used        float64 `json:"used"`
			Available   float64 `json:"available"`
			Period      string  `json:"period"`
			Description string  `json:"description"`
		} `json:"data"`
	}

	req, err := a.buildRequest(ctx, session, "POST", "/api/expense/budget/types", map[string]any{})
	if err != nil {
		return nil, err
	}

	var resp budgetResponse
	if err := a.doRequest(req, &resp); err != nil {
		return nil, err
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("获取费用预算类型失败: %s", resp.Message)
	}

	candidates := make([]target.ExpenseBudgetType, 0, len(resp.Data))
	for _, item := range resp.Data {
		candidates = append(candidates, target.ExpenseBudgetType{
			ID:          item.ID,
			Name:        item.Name,
			Code:        item.Code,
			Budget:      item.Budget,
			Used:        item.Used,
			Available:   item.Available,
			Period:      item.Period,
			Description: item.Description,
		})
	}

	return candidates, nil
}

// GetCityCandidates 获取城市选择候选项（静态数据）。
func (a *ComponentCandidateAdapter) GetCityCandidates(ctx context.Context, account string) ([]target.CityCandidate, error) {
	// 城市数据通常是静态的，可以从本地配置或远程获取
	session, err := a.client.session(ctx, account)
	if err != nil {
		return nil, err
	}

	type cityResponse struct {
		IsSuccess bool   `json:"isSuccess"`
		Message   string `json:"message"`
		Data      []struct {
			Code       string `json:"code"`
			Name       string `json:"name"`
			Province   string `json:"province"`
			Level      string `json:"level"`
			PinYin     string `json:"pinyin"`
			ParentCode string `json:"parentCode"`
		} `json:"data"`
	}

	req, err := a.buildRequest(ctx, session, "POST", "/api/common/cities", map[string]any{})
	if err != nil {
		return nil, err
	}

	var resp cityResponse
	if err := a.doRequest(req, &resp); err != nil {
		return nil, err
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("获取城市候选失败: %s", resp.Message)
	}

	candidates := make([]target.CityCandidate, 0, len(resp.Data))
	for _, item := range resp.Data {
		candidates = append(candidates, target.CityCandidate{
			Code:       item.Code,
			Name:       item.Name,
			Province:   item.Province,
			Level:      item.Level,
			PinYin:     item.PinYin,
			ParentCode: item.ParentCode,
		})
	}

	return candidates, nil
}

// GetTravelRoutes 获取差旅路线候选项。
func (a *ComponentCandidateAdapter) GetTravelRoutes(ctx context.Context, account string) ([]target.TravelRoute, error) {
	session, err := a.client.session(ctx, account)
	if err != nil {
		return nil, err
	}

	type routeResponse struct {
		IsSuccess bool   `json:"isSuccess"`
		Message   string `json:"message"`
		Data      []struct {
			ID            string  `json:"id"`
			StartCity     string  `json:"startCity"`
			EndCity       string  `json:"endCity"`
			Distance      float64 `json:"distance"`
			EstimatedDays int     `json:"estimatedDays"`
			TransportMode string  `json:"transportMode"`
			EstimatedCost float64 `json:"estimatedCost"`
		} `json:"data"`
	}

	req, err := a.buildRequest(ctx, session, "POST", "/api/travel/routes", map[string]any{
		"page":     1,
		"pageSize": 100,
	})
	if err != nil {
		return nil, err
	}

	var resp routeResponse
	if err := a.doRequest(req, &resp); err != nil {
		return nil, err
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("获取差旅路线失败: %s", resp.Message)
	}

	candidates := make([]target.TravelRoute, 0, len(resp.Data))
	for _, item := range resp.Data {
		candidates = append(candidates, target.TravelRoute{
			ID:            item.ID,
			StartCity:     item.StartCity,
			EndCity:       item.EndCity,
			Distance:      item.Distance,
			EstimatedDays: item.EstimatedDays,
			TransportMode: item.TransportMode,
			EstimatedCost: item.EstimatedCost,
		})
	}

	return candidates, nil
}

// buildRequest 构建带会话的 HTTP 请求。
func (a *ComponentCandidateAdapter) buildRequest(ctx context.Context, session *target.Session, method, endpoint string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = strings.NewReader(string(data))
	}

	url := a.client.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if session.SID != "" {
		req.Header.Set("Cookie", "sid="+session.SID)
	}

	return req, nil
}

// doRequest 执行 HTTP 请求并解析响应。
func (a *ComponentCandidateAdapter) doRequest(req *http.Request, result any) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, result)
}

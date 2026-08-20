package target

import "context"

// ComponentCandidateProvider 为自定义组件提供按发起人权限筛选的真实候选对象。
type ComponentCandidateProvider interface {
	// GetMaterialCandidates 获取材料选择组件的候选项
	GetMaterialCandidates(ctx context.Context, account, flowCode, direction string) ([]MaterialCandidate, error)

	// GetProjectCandidates 获取项目选择组件的候选项
	GetProjectCandidates(ctx context.Context, account, flowCode string) ([]ProjectCandidate, error)

	// GetOrderCandidates 获取订单选择组件的候选项
	GetOrderCandidates(ctx context.Context, account, flowCode string) ([]OrderCandidate, error)

	// GetFlowListCandidates 获取流程列表选择组件的候选项
	GetFlowListCandidates(ctx context.Context, account, flowCode string) ([]FlowListCandidate, error)

	// GetExpenseBudgetTypes 获取费用预算类型候选项
	GetExpenseBudgetTypes(ctx context.Context, account string) ([]ExpenseBudgetType, error)

	// GetCityCandidates 获取城市选择候选项
	GetCityCandidates(ctx context.Context, account string) ([]CityCandidate, error)

	// GetTravelRoutes 获取差旅路线候选项
	GetTravelRoutes(ctx context.Context, account string) ([]TravelRoute, error)
}

// MaterialCandidate 是材料选择组件的候选对象。
type MaterialCandidate struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Code          string  `json:"code"`
	Specification string  `json:"specification"`
	Unit          string  `json:"unit"`
	Price         float64 `json:"price"`
	Category      string  `json:"category"`
}

// ProjectCandidate 是项目选择组件的候选对象。
type ProjectCandidate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Manager     string `json:"manager"`
	CompanyID   string `json:"companyId"`
	CompanyName string `json:"companyName"`
}

// OrderCandidate 是订单选择组件的候选对象。
type OrderCandidate struct {
	ID          string  `json:"id"`
	OrderNo     string  `json:"orderNo"`
	Type        string  `json:"type"`
	Amount      float64 `json:"amount"`
	Status      string  `json:"status"`
	CreateDate  string  `json:"createDate"`
	Description string  `json:"description"`
}

// FlowListCandidate 是流程列表选择组件的候选对象。
type FlowListCandidate struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	DeptID     string `json:"deptId"`
	DeptName   string `json:"deptName"`
	CompanyID  string `json:"companyId"`
	CompanyName string `json:"companyName"`
}

// ExpenseBudgetType 是费用预算类型候选对象。
type ExpenseBudgetType struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Code        string  `json:"code"`
	Budget      float64 `json:"budget"`
	Used        float64 `json:"used"`
	Available   float64 `json:"available"`
	Period      string  `json:"period"`
	Description string  `json:"description"`
}

// CityCandidate 是城市选择候选对象。
type CityCandidate struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Province   string `json:"province"`
	Level      string `json:"level"`
	PinYin     string `json:"pinyin"`
	ParentCode string `json:"parentCode"`
}

// TravelRoute 是差旅路线候选对象。
type TravelRoute struct {
	ID            string  `json:"id"`
	StartCity     string  `json:"startCity"`
	EndCity       string  `json:"endCity"`
	Distance      float64 `json:"distance"`
	EstimatedDays int     `json:"estimatedDays"`
	TransportMode string  `json:"transportMode"`
	EstimatedCost float64 `json:"estimatedCost"`
}

// ComponentCandidateSet 是为一个流程模板预加载的全部组件候选集合。
type ComponentCandidateSet struct {
	FlowCode          string
	Account           string
	RuleVersion       string
	Materials         map[string][]MaterialCandidate  // 按方向分组：in/out
	Projects          []ProjectCandidate
	Orders            []OrderCandidate
	FlowLists         []FlowListCandidate
	ExpenseBudgetTypes []ExpenseBudgetType
	Cities            []CityCandidate
	TravelRoutes      []TravelRoute
}

// SerializeCandidate 将候选对象序列化为组件要求的 JSON 字符串。
func SerializeCandidate(candidate any) string {
	// 实际实现在 formdata 包中
	return ""
}

package apifox

import (
	"encoding/json"
)

// ApiTreeListResponse API树形列表响应结构
type ApiTreeListResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

// ApiTreeItem API树形列表项
type ApiTreeItem struct {
	Key      string          `json:"key"`
	Type     string          `json:"type"`
	Name     string          `json:"name"`
	Children json.RawMessage `json:"children"`
	Api      *ApiBasic       `json:"api,omitempty"`
	Folder   *ApiFolder      `json:"folder,omitempty"`
}

// ApiFolder API文件夹信息
type ApiFolder struct {
	ID              int         `json:"id"`
	Name            string      `json:"name"`
	DocID           int         `json:"docId"`
	ParentID        int         `json:"parentId"`
	ProjectBranchID int         `json:"projectBranchId"`
	ShareSettings   interface{} `json:"shareSettings"`
	Visibility      string      `json:"visibility"`
	EditorID        int         `json:"editorId"`
	Type            string      `json:"type"`
}

// ApiBasic API基本信息
type ApiBasic struct {
	ID              int         `json:"id"`
	Name            string      `json:"name"`
	Type            string      `json:"type"`
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	FolderID        int         `json:"folderId"`
	Tags            []string    `json:"tags"`
	Status          string      `json:"status"`
	ResponsibleID   int         `json:"responsibleId"`
	CustomApiFields interface{} `json:"customApiFields"`
	Visibility      string      `json:"visibility"`
	EditorID        int         `json:"editorId"`
}

// ApiDetailResponse API详细信息响应结构
type ApiDetailResponse struct {
	Success bool      `json:"success"`
	Data    ApiDetail `json:"data"`
}

// ApiDetail API详细信息
type ApiDetail struct {
	ID               int              `json:"id"`
	Name             string           `json:"name"`
	Type             string           `json:"type"`
	Method           string           `json:"method"`
	Path             string           `json:"path"`
	Description      string           `json:"description"`
	Status           string           `json:"status"`
	RequestBody      RequestBody      `json:"requestBody"`
	Parameters       Parameters       `json:"parameters"`
	Responses        []Response       `json:"responses"`
	FolderID         int              `json:"folderId"`
	Tags             []string         `json:"tags"`
	CreatedAt        string           `json:"createdAt"`
	UpdatedAt        string           `json:"updatedAt"`
	CreatorID        int              `json:"creatorId"`
	EditorID         int              `json:"editorId"`
	OperationID      string           `json:"operationId"`
	CommonParameters CommonParameters `json:"commonParameters"`
	Visibility       string           `json:"visibility"`
}

// RequestBody 请求体
type RequestBody struct {
	Type       string        `json:"type"`
	Parameters []Parameter   `json:"parameters"`
	JsonSchema interface{}   `json:"jsonSchema"`
	MediaType  string        `json:"mediaType"`
	Examples   []interface{} `json:"examples"`
}

// Parameters 参数信息
type Parameters struct {
	Query []Parameter `json:"query"`
	Path  []Parameter `json:"path"`
}

// Parameter 参数详情
type Parameter struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Enable      bool   `json:"enable"`
}

// Response 响应详情
type Response struct {
	ID          int           `json:"id"`
	Name        string        `json:"name"`
	Code        int           `json:"code"`
	ContentType string        `json:"contentType"`
	JsonSchema  interface{}   `json:"jsonSchema"`
	Description string        `json:"description"`
	Headers     []interface{} `json:"headers"`
}

// CommonParameters 通用参数
type CommonParameters struct {
	Query  []interface{} `json:"query"`
	Body   []interface{} `json:"body"`
	Cookie []interface{} `json:"cookie"`
	Header []HeaderParam `json:"header"`
}

// HeaderParam 头部参数
type HeaderParam struct {
	Name string `json:"name"`
}

// WebhookPayload 接收到的Webhook请求体
type WebhookPayload struct {
	Event   string `json:"event"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// StoredApiInfo 存储的API信息
type StoredApiInfo struct {
	ApiPath   string    `json:"api_path"`
	ApiKey    string    `json:"api_key"`
	ApiID     int       `json:"api_id"`
	Name      string    `json:"name"`
	Method    string    `json:"method"`
	Detail    ApiDetail `json:"detail"`
	UpdatedAt string    `json:"updated_at"`
}

// ApiDiff API差异信息
type ApiDiff struct {
	ApiKey     string `json:"api_key"`
	ApiID      int    `json:"api_id"`
	Name       string `json:"name"`
	Method     string `json:"method"`
	OldMethod  string `json:"old_method"`
	OldPath    string `json:"old_path"`
	NewPath    string `json:"new_path"`
	PathDiff   bool   `json:"path_diff"`
	MethodDiff bool   `json:"method_diff"`

	RequestBodyDiff   bool   `json:"request_body_diff"`
	RequestBodyDetail string `json:"request_body_detail,omitempty"`

	ParametersDiff   bool   `json:"parameters_diff"`
	ParametersDetail string `json:"parameters_detail,omitempty"`

	ResponsesDiff   bool   `json:"responses_diff"`
	ResponsesDetail string `json:"responses_detail,omitempty"`

	ModifierName string `json:"modifier_name"`
	ModifiedTime string `json:"modified_time"`
}

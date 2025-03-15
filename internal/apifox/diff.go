package apifox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// DiffService API 差异比较服务
type DiffService struct {
	logger *logrus.Logger
}

// NewDiffService 创建新的差异比较服务
func NewDiffService(logger *logrus.Logger) *DiffService {
	return &DiffService{
		logger: logger,
	}
}

// CompareApis 比较两个 API 的差异
func (s *DiffService) CompareApis(oldApi, newApi ApiDetail, modifierName, modifiedTime string) *ApiDiff {
	diff := &ApiDiff{
		ApiID:        newApi.ID,
		ApiKey:       fmt.Sprintf("apiDetail.%d", newApi.ID),
		Name:         newApi.Name,
		Method:       newApi.Method,
		OldMethod:    oldApi.Method,
		OldPath:      oldApi.Path,
		NewPath:      newApi.Path,
		ModifierName: modifierName,
		ModifiedTime: modifiedTime,
	}

	// 比较HTTP方法
	diff.MethodDiff = strings.ToLower(oldApi.Method) != strings.ToLower(newApi.Method)

	// 比较路径
	diff.PathDiff = oldApi.Path != newApi.Path

	// 比较请求体 - 详细分析变更内容
	oldRequestBodyJSON, _ := json.Marshal(oldApi.RequestBody)
	newRequestBodyJSON, _ := json.Marshal(newApi.RequestBody)
	diff.RequestBodyDiff = !bytes.Equal(oldRequestBodyJSON, newRequestBodyJSON)

	if diff.RequestBodyDiff {
		var rbDetails strings.Builder
		rbDetails.WriteString("【请求体变更】\n")

		// 检查内容类型变更 - 处理空值和none的情况
		oldMediaType := oldApi.RequestBody.MediaType
		newMediaType := newApi.RequestBody.MediaType
		if oldMediaType == "" {
			oldMediaType = "none"
		}
		if newMediaType == "" {
			newMediaType = "none"
		}

		if oldMediaType != newMediaType {
			rbDetails.WriteString(fmt.Sprintf("* 请求体类型: %s -> %s\n", oldMediaType, newMediaType))
		}

		// 检查请求体类型变更
		if oldApi.RequestBody.Type != newApi.RequestBody.Type {
			rbDetails.WriteString(fmt.Sprintf("* 请求体类型: %s -> %s\n", oldApi.RequestBody.Type, newApi.RequestBody.Type))
		}

		// 检查请求体结构(JsonSchema)变更
		oldSchemaJSON, _ := json.Marshal(oldApi.RequestBody.JsonSchema)
		newSchemaJSON, _ := json.Marshal(newApi.RequestBody.JsonSchema)
		if !bytes.Equal(oldSchemaJSON, newSchemaJSON) {
			// 直接分析JSON结构变化
			if err := analyzeJsonSchemaDiff(&rbDetails, oldApi.RequestBody.JsonSchema, newApi.RequestBody.JsonSchema); err != nil {
				s.logger.WithError(err).Warn("分析请求体JSON结构变化失败")
			}
		}

		// 比较请求体参数
		oldParamMap := make(map[string]Parameter)
		for _, p := range oldApi.RequestBody.Parameters {
			oldParamMap[p.Name] = p
		}

		// 检查新增或修改的参数
		hasParamChanges := false
		for _, newParam := range newApi.RequestBody.Parameters {
			oldParam, exists := oldParamMap[newParam.Name]
			if !exists {
				// 新增的参数
				rbDetails.WriteString(fmt.Sprintf("+ 新增参数: %s (%s", newParam.Name, newParam.Type))
				if newParam.Required {
					rbDetails.WriteString(", 必填")
				}
				rbDetails.WriteString(")\n")
				hasParamChanges = true
			} else {
				// 检查参数是否有变化
				if newParam.Type != oldParam.Type || newParam.Required != oldParam.Required ||
					newParam.Description != oldParam.Description || newParam.Enable != oldParam.Enable {
					rbDetails.WriteString(fmt.Sprintf("* 修改参数: %s\n", newParam.Name))

					if newParam.Type != oldParam.Type {
						rbDetails.WriteString(fmt.Sprintf("  - 类型: %s -> %s\n", oldParam.Type, newParam.Type))
					}

					if newParam.Required != oldParam.Required {
						if newParam.Required {
							rbDetails.WriteString("  - 变为必填\n")
						} else {
							rbDetails.WriteString("  - 变为非必填\n")
						}
					}

					if newParam.Description != oldParam.Description {
						rbDetails.WriteString(fmt.Sprintf("  - 描述变更: %s -> %s\n", oldParam.Description, newParam.Description))
					}

					if newParam.Enable != oldParam.Enable {
						if newParam.Enable {
							rbDetails.WriteString("  - 已启用\n")
						} else {
							rbDetails.WriteString("  - 已禁用\n")
						}
					}

					hasParamChanges = true
				}
			}

			// 从旧参数映射中删除已处理的参数
			delete(oldParamMap, newParam.Name)
		}

		// 检查已删除的参数
		for name, param := range oldParamMap {
			rbDetails.WriteString(fmt.Sprintf("- 删除参数: %s (%s)\n", name, param.Type))
			hasParamChanges = true
		}

		if hasParamChanges {
			rbDetails.WriteString("\n")
		}

		diff.RequestBodyDetail = rbDetails.String()
	}

	// 比较参数 - 详细分析变更内容
	oldParamsJSON, _ := json.Marshal(oldApi.Parameters)
	newParamsJSON, _ := json.Marshal(newApi.Parameters)
	diff.ParametersDiff = !bytes.Equal(oldParamsJSON, newParamsJSON)

	if diff.ParametersDiff {
		var paramDetails strings.Builder

		// 比较查询参数(Query Parameters)
		paramDetails.WriteString("【查询参数(Query)变更】\n")
		hasQueryChanges := false

		// 创建旧参数的映射，用于快速查找
		oldQueryParams := make(map[string]Parameter)
		for _, p := range oldApi.Parameters.Query {
			oldQueryParams[p.Name] = p
		}

		// 检查新增或修改的参数
		for _, newParam := range newApi.Parameters.Query {
			oldParam, exists := oldQueryParams[newParam.Name]
			if !exists {
				// 新增的参数
				paramDetails.WriteString(fmt.Sprintf("+ 新增: %s (%s", newParam.Name, newParam.Type))
				if newParam.Required {
					paramDetails.WriteString(", 必填")
				}
				paramDetails.WriteString(")\n")
				hasQueryChanges = true
			} else {
				// 检查参数是否有变化
				if newParam.Type != oldParam.Type || newParam.Required != oldParam.Required ||
					newParam.Description != oldParam.Description || newParam.Enable != oldParam.Enable {
					paramDetails.WriteString(fmt.Sprintf("* 修改: %s\n", newParam.Name))
					if newParam.Type != oldParam.Type {
						paramDetails.WriteString(fmt.Sprintf("  - 类型: %s -> %s\n", oldParam.Type, newParam.Type))
					}
					if newParam.Required != oldParam.Required {
						if newParam.Required {
							paramDetails.WriteString("  - 变为必填\n")
						} else {
							paramDetails.WriteString("  - 变为非必填\n")
						}
					}
					if newParam.Description != oldParam.Description {
						paramDetails.WriteString(fmt.Sprintf("  - 描述变更: %s -> %s\n", oldParam.Description, newParam.Description))
					}
					if newParam.Enable != oldParam.Enable {
						if newParam.Enable {
							paramDetails.WriteString("  - 已启用\n")
						} else {
							paramDetails.WriteString("  - 已禁用\n")
						}
					}
					hasQueryChanges = true
				}
			}

			// 从老参数映射中删除已处理的参数
			delete(oldQueryParams, newParam.Name)
		}

		// 检查已删除的参数 - 直接遍历剩余的oldQueryParams即可找到被删除的参数
		for name, param := range oldQueryParams {
			paramDetails.WriteString(fmt.Sprintf("- 删除: %s (%s)\n", name, param.Type))
			hasQueryChanges = true
		}

		if !hasQueryChanges {
			paramDetails.WriteString("无变更\n")
		}

		// 比较路径参数(Path Parameters)
		paramDetails.WriteString("\n【路径参数(Path)变更】\n")
		hasPathChanges := false

		// 创建旧参数的映射
		oldPathParams := make(map[string]Parameter)
		for _, p := range oldApi.Parameters.Path {
			oldPathParams[p.Name] = p
		}

		// 检查新增或修改的参数
		for _, newParam := range newApi.Parameters.Path {
			oldParam, exists := oldPathParams[newParam.Name]
			if !exists {
				// 新增的参数
				paramDetails.WriteString(fmt.Sprintf("+ 新增: %s (%s", newParam.Name, newParam.Type))
				if newParam.Required {
					paramDetails.WriteString(", 必填")
				}
				paramDetails.WriteString(")\n")
				hasPathChanges = true
			} else {
				// 检查参数是否有变化
				if newParam.Type != oldParam.Type || newParam.Required != oldParam.Required ||
					newParam.Description != oldParam.Description || newParam.Enable != oldParam.Enable {
					paramDetails.WriteString(fmt.Sprintf("* 修改: %s\n", newParam.Name))
					if newParam.Type != oldParam.Type {
						paramDetails.WriteString(fmt.Sprintf("  - 类型: %s -> %s\n", oldParam.Type, newParam.Type))
					}
					if newParam.Required != oldParam.Required {
						if newParam.Required {
							paramDetails.WriteString("  - 变为必填\n")
						} else {
							paramDetails.WriteString("  - 变为非必填\n")
						}
					}
					if newParam.Description != oldParam.Description {
						paramDetails.WriteString(fmt.Sprintf("  - 描述变更: %s -> %s\n", oldParam.Description, newParam.Description))
					}
					if newParam.Enable != oldParam.Enable {
						if newParam.Enable {
							paramDetails.WriteString("  - 已启用\n")
						} else {
							paramDetails.WriteString("  - 已禁用\n")
						}
					}
					hasPathChanges = true
				}
			}

			// 从老参数映射中删除已处理的参数
			delete(oldPathParams, newParam.Name)
		}

		// 检查已删除的参数 - 直接遍历剩余的oldPathParams即可找到被删除的参数
		for name, param := range oldPathParams {
			paramDetails.WriteString(fmt.Sprintf("- 删除: %s (%s)\n", name, param.Type))
			hasPathChanges = true
		}

		if !hasPathChanges {
			paramDetails.WriteString("无变更\n")
		}

		diff.ParametersDetail = paramDetails.String()
	}

	// 比较响应
	oldResponsesJSON, _ := json.Marshal(oldApi.Responses)
	newResponsesJSON, _ := json.Marshal(newApi.Responses)
	diff.ResponsesDiff = !bytes.Equal(oldResponsesJSON, newResponsesJSON)

	if diff.ResponsesDiff {
		var respDetails strings.Builder
		respDetails.WriteString("【响应状态码变更】\n")

		// 建立旧响应的映射，以状态码为键
		oldResponseMap := make(map[int]Response)
		for _, resp := range oldApi.Responses {
			oldResponseMap[resp.Code] = resp
		}

		// 检查新增或修改的响应
		for _, newResp := range newApi.Responses {
			oldResp, exists := oldResponseMap[newResp.Code]
			if !exists {
				// 新增的响应状态码
				respDetails.WriteString(fmt.Sprintf("+ 新增状态码: %d (%s)\n", newResp.Code, newResp.Name))
			} else {
				// 检查响应内容是否变化
				oldRespJSON, _ := json.Marshal(oldResp)
				newRespJSON, _ := json.Marshal(newResp)

				if !bytes.Equal(oldRespJSON, newRespJSON) {
					respDetails.WriteString(fmt.Sprintf("* 修改状态码: %d\n", newResp.Code))

					// 检查名称变更
					if oldResp.Name != newResp.Name {
						respDetails.WriteString(fmt.Sprintf("  - 名称: %s -> %s\n", oldResp.Name, newResp.Name))
					}

					// 检查内容类型变更
					if oldResp.ContentType != newResp.ContentType {
						respDetails.WriteString(fmt.Sprintf("  - 内容类型: %s -> %s\n", oldResp.ContentType, newResp.ContentType))
					}

					// 检查描述变更
					if oldResp.Description != newResp.Description {
						respDetails.WriteString(fmt.Sprintf("  - 描述变更\n"))
					}

					// 检查JSON结构变更
					oldSchemaJSON, _ := json.Marshal(oldResp.JsonSchema)
					newSchemaJSON, _ := json.Marshal(newResp.JsonSchema)
					if !bytes.Equal(oldSchemaJSON, newSchemaJSON) {
						respDetails.WriteString("  - 响应结构变更\n")
					}
				}
			}

			// 从旧响应映射中删除已处理的状态码
			delete(oldResponseMap, newResp.Code)
		}

		// 检查已删除的响应状态码
		for code, resp := range oldResponseMap {
			respDetails.WriteString(fmt.Sprintf("- 删除状态码: %d (%s)\n", code, resp.Name))
		}

		diff.ResponsesDetail = respDetails.String()
	}

	return diff
}

// ParseWebhookContent 解析 webhook 内容以获取 API 信息
func ParseWebhookContent(content string) (string, string, error) {
	lines := strings.Split(content, "\n")
	var apiName, apiPath string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "接口名称：") {
			apiName = strings.TrimPrefix(line, "接口名称：")
		} else if strings.HasPrefix(line, "接口路径：") {
			apiPath = strings.TrimPrefix(line, "接口路径：")
		}
	}

	if apiName == "" || apiPath == "" {
		return "", "", fmt.Errorf("webhook 内容中未找到接口名称或路径信息")
	}

	return apiName, apiPath, nil
}

// ExtractApiKeyFromTreeItem 从 API 树形列表项中提取 API Key
func ExtractApiKeyFromTreeItem(apiName string, items []ApiTreeItem) (string, error) {
	for _, item := range items {
		if item.Type == "apiDetail" && item.Name == apiName {
			return item.Key, nil
		}

		// 递归搜索子项
		if len(item.Children) > 0 {
			// 先尝试解析为 ApiTreeItem 数组
			var children []ApiTreeItem
			if err := json.Unmarshal(item.Children, &children); err == nil {
				// 递归检查子项
				for _, child := range children {
					if child.Type == "apiDetail" && child.Name == apiName {
						return child.Key, nil
					}

					// 再次递归检查这个子项的子项
					if len(child.Children) > 0 {
						childKey, err := ExtractApiKeyFromTreeItem(apiName, []ApiTreeItem{child})
						if err == nil {
							return childKey, nil
						}
					}
				}
			} else {
				// 如果解析失败，尝试解析为通用 map 数组
				var genericChildren []map[string]interface{}
				if err := json.Unmarshal(item.Children, &genericChildren); err == nil {
					for _, child := range genericChildren {
						// 检查是否为 apiDetail 类型
						if childType, ok := child["type"].(string); ok && childType == "apiDetail" {
							// 检查名称是否匹配
							if childName, ok := child["name"].(string); ok && childName == apiName {
								// 返回找到的 key
								if key, ok := child["key"].(string); ok {
									return key, nil
								}
							}
						}

						// 检查这个子项是否有子项
						if childrenRaw, ok := child["children"]; ok {
							// 将子项的 children 转为 JSON
							childrenJSON, err := json.Marshal(childrenRaw)
							if err == nil && len(childrenJSON) > 2 { // 检查 JSON 是否不为 [] 或 {}
								// 创建一个临时 ApiTreeItem 以进行递归
								tempItem := ApiTreeItem{
									Children: childrenJSON,
								}

								childKey, err := ExtractApiKeyFromTreeItem(apiName, []ApiTreeItem{tempItem})
								if err == nil {
									return childKey, nil
								}
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("在 API 树形列表中未找到名为 '%s' 的 API", apiName)
}

// ExtractMethodFromPath 从路径中提取 HTTP 方法
func ExtractMethodFromPath(path string) string {
	parts := strings.Split(path, " ")
	if len(parts) > 0 {
		return strings.ToLower(parts[0])
	}
	return ""
}

// FormatCurrentTime 格式化当前时间
func FormatCurrentTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// analyzeJsonSchemaDiff 分析JSON Schema的变化并生成详细说明
func analyzeJsonSchemaDiff(builder *strings.Builder, oldSchema, newSchema interface{}) error {
	// 如果两者都为nil或者空字符串，则没有变化
	if oldSchema == nil && newSchema == nil {
		return nil
	}

	// 如果其中一个为nil而另一个不是，则记录变化
	// 但是不再显示"添加了请求体结构"，因为外层已经显示了请求体类型变更
	if oldSchema == nil && newSchema != nil {
		// 分析新添加的结构，提供更具体的信息
		newMap, newIsMap := newSchema.(map[string]interface{})
		if newIsMap {
			// 分析属性变化
			if newProps, ok := newMap["properties"].(map[string]interface{}); ok && len(newProps) > 0 {
				// 因为是新增结构，所以直接显示所有字段
				for propName, prop := range newProps {
					propType := "object"
					if propMap, ok := prop.(map[string]interface{}); ok {
						if t, ok := propMap["type"].(string); ok {
							propType = t
						}
					}
					builder.WriteString(fmt.Sprintf("* 新增字段: %s (%s)\n", propName, propType))
				}

				// 分析必填项
				if required, ok := newMap["required"].([]interface{}); ok && len(required) > 0 {
					builder.WriteString("* 必填字段:\n")
					for _, field := range required {
						if fieldStr, ok := field.(string); ok {
							builder.WriteString(fmt.Sprintf("  - %s\n", fieldStr))
						}
					}
				}
			} else {
				// 没有属性的情况（比如空的JSON对象）
				if schemaType, ok := newMap["type"].(string); ok {
					if schemaType == "object" {
						// 对象类型但没有属性，说明是空对象
						// 不输出任何内容，避免冗余信息
					} else {
						// 其他类型（如string、number等）
						builder.WriteString(fmt.Sprintf("* 请求体为 %s 类型\n", schemaType))
					}
				}
			}
		} else {
			// 非对象类型，可能是简单值
			if newStr, ok := newSchema.(string); ok && newStr != "" {
				builder.WriteString(fmt.Sprintf("* 请求体值: %s\n", newStr))
			}
		}
		return nil
	}

	if oldSchema != nil && newSchema == nil {
		builder.WriteString("* 移除了请求体结构\n")
		return nil
	}

	// 对复杂结构进行分析
	oldMap, oldIsMap := oldSchema.(map[string]interface{})
	newMap, newIsMap := newSchema.(map[string]interface{})

	if oldIsMap && newIsMap {
		// 分析类型变化
		if oldType, ok := oldMap["type"].(string); ok {
			if newType, ok := newMap["type"].(string); ok {
				if oldType != newType {
					builder.WriteString(fmt.Sprintf("* 数据类型: %s -> %s\n", oldType, newType))
				}
			}
		}

		// 分析属性变化
		if oldProps, ok := oldMap["properties"].(map[string]interface{}); ok {
			if newProps, ok := newMap["properties"].(map[string]interface{}); ok {
				// 比较属性
				analyzePropertiesDiff(builder, oldProps, newProps)
			}
		}

		// 分析必填项变化 - 避免与字段变化重复
		if oldRequired, ok := oldMap["required"].([]interface{}); ok {
			if newRequired, ok := newMap["required"].([]interface{}); ok {
				oldReqSlice := interfaceSliceToStringSlice(oldRequired)
				newReqSlice := interfaceSliceToStringSlice(newRequired)

				// 只处理那些添加/删除的字段不包含的必填变更
				if !equalStringSlices(oldReqSlice, newReqSlice) {
					// 创建属性映射，用于过滤那些已经在属性变更中提到的字段
					oldPropsMap := make(map[string]bool)
					newPropsMap := make(map[string]bool)

					if oldProps, ok := oldMap["properties"].(map[string]interface{}); ok {
						for propName := range oldProps {
							oldPropsMap[propName] = true
						}
					}

					if newProps, ok := newMap["properties"].(map[string]interface{}); ok {
						for propName := range newProps {
							newPropsMap[propName] = true
						}
					}

					// 找出必填项变化但字段未变化的项
					hasRequiredChanges := false

					// 新增的必填项
					for _, field := range newReqSlice {
						if !contains(oldReqSlice, field) && oldPropsMap[field] && newPropsMap[field] {
							if !hasRequiredChanges {
								builder.WriteString("* 必填项变更:\n")
								hasRequiredChanges = true
							}
							builder.WriteString(fmt.Sprintf("  + 新增必填: %s\n", field))
						}
					}

					// 移除的必填项
					for _, field := range oldReqSlice {
						if !contains(newReqSlice, field) && oldPropsMap[field] && newPropsMap[field] {
							if !hasRequiredChanges {
								builder.WriteString("* 必填项变更:\n")
								hasRequiredChanges = true
							}
							builder.WriteString(fmt.Sprintf("  - 移除必填: %s\n", field))
						}
					}
				}
			}
		}

		return nil
	}

	// 如果是简单类型，直接比较
	oldStr, oldIsStr := oldSchema.(string)
	newStr, newIsStr := newSchema.(string)

	if oldIsStr && newIsStr && oldStr != newStr {
		builder.WriteString(fmt.Sprintf("* 值变更: %s -> %s\n", oldStr, newStr))
		return nil
	}

	return nil
}

// analyzePropertiesDiff 分析属性的变化
func analyzePropertiesDiff(builder *strings.Builder, oldProps, newProps map[string]interface{}) {
	// 记录删除的字段（仅真正删除的字段，而非修改的字段）
	var removedFields []string

	// 记录新增的字段（仅真正新增的字段，而非修改的字段）
	var addedFields []string

	// 记录修改的字段
	var modifiedFields []struct {
		name              string
		oldType           string
		newType           string
		oldTitle          string
		newTitle          string
		oldDesc           string
		newDesc           string
		oldRequired       bool
		newRequired       bool
		hasRequiredChange bool
		changes           map[string]struct{ old, new interface{} }
	}

	// 获取必填字段列表
	oldRequiredFields := make(map[string]bool)
	newRequiredFields := make(map[string]bool)

	// 从父级schema获取required字段列表
	if oldRequiredList, ok := oldProps["required"].([]interface{}); ok {
		for _, field := range oldRequiredList {
			if fieldName, ok := field.(string); ok {
				oldRequiredFields[fieldName] = true
			}
		}
	}

	if newRequiredList, ok := newProps["required"].([]interface{}); ok {
		for _, field := range newRequiredList {
			if fieldName, ok := field.(string); ok {
				newRequiredFields[fieldName] = true
			}
		}
	}

	// 首先找出在两个集合中都存在的字段(可能被修改)和只在一个集合中存在的字段(新增或删除)
	for propName, oldProp := range oldProps {
		if propName == "required" {
			continue // 跳过required字段，它会在字段级别处理
		}

		if newProp, exists := newProps[propName]; exists {
			// 字段在新旧两个集合中都存在，检查是否有变化
			oldPropJSON, _ := json.Marshal(oldProp)
			newPropJSON, _ := json.Marshal(newProp)

			// 检查必填状态变化
			oldRequired := oldRequiredFields[propName]
			newRequired := newRequiredFields[propName]
			hasRequiredChange := oldRequired != newRequired

			if !bytes.Equal(oldPropJSON, newPropJSON) || hasRequiredChange {
				// 检测到变化，这是一个修改的字段
				var oldType, newType string
				var oldTitle, newTitle string
				var oldDesc, newDesc string
				changes := make(map[string]struct{ old, new interface{} })

				// 提取旧属性
				if oldPropMap, ok := oldProp.(map[string]interface{}); ok {
					if t, ok := oldPropMap["type"].(string); ok {
						oldType = t
					}
					if t, ok := oldPropMap["title"].(string); ok {
						oldTitle = t
					}
					if d, ok := oldPropMap["description"].(string); ok {
						oldDesc = d
					}

					// 检查其他属性变化
					if newPropMap, ok := newProp.(map[string]interface{}); ok {
						// 提取新属性
						if t, ok := newPropMap["type"].(string); ok {
							newType = t
						}
						if t, ok := newPropMap["title"].(string); ok {
							newTitle = t
						}
						if d, ok := newPropMap["description"].(string); ok {
							newDesc = d
						}

						for k, v := range oldPropMap {
							if newV, exists := newPropMap[k]; exists && !reflect.DeepEqual(v, newV) {
								changes[k] = struct{ old, new interface{} }{v, newV}
							}
						}

						// 也检查新属性中存在但旧属性中不存在的键
						for k, v := range newPropMap {
							if _, exists := oldPropMap[k]; !exists {
								changes[k] = struct{ old, new interface{} }{nil, v}
							}
						}
					}
				}

				// 添加到修改字段列表
				modifiedFields = append(modifiedFields, struct {
					name              string
					oldType           string
					newType           string
					oldTitle          string
					newTitle          string
					oldDesc           string
					newDesc           string
					oldRequired       bool
					newRequired       bool
					hasRequiredChange bool
					changes           map[string]struct{ old, new interface{} }
				}{
					propName,
					oldType,
					newType,
					oldTitle,
					newTitle,
					oldDesc,
					newDesc,
					oldRequired,
					newRequired,
					hasRequiredChange,
					changes,
				})
			}
		} else {
			// 字段在旧集合中存在但在新集合中不存在，是真正删除的字段
			removedFields = append(removedFields, propName)
		}
	}

	// 找出真正新增的字段（只在新集合中存在）
	for propName := range newProps {
		if propName == "required" {
			continue // 跳过required字段，它会在字段级别处理
		}

		if _, exists := oldProps[propName]; !exists {
			addedFields = append(addedFields, propName)
		}
	}

	// 先显示字段删除
	if len(removedFields) > 0 {
		for _, name := range removedFields {
			oldProp := oldProps[name]
			builder.WriteString(fmt.Sprintf("* 删除字段: %s", name))

			// 尝试添加类型信息
			if oldPropMap, ok := oldProp.(map[string]interface{}); ok {
				if propType, ok := oldPropMap["type"].(string); ok {
					builder.WriteString(fmt.Sprintf(" (%s)", propType))
				}

				// 添加中文名称信息
				if title, ok := oldPropMap["title"].(string); ok && title != "" {
					builder.WriteString(fmt.Sprintf(" [%s]", title))
				}
			}

			// 添加必填信息
			if oldRequiredFields[name] {
				builder.WriteString(" (必填)")
			}

			builder.WriteString("\n")
		}
	}

	// 再显示字段新增
	if len(addedFields) > 0 {
		for _, name := range addedFields {
			newProp := newProps[name]
			builder.WriteString(fmt.Sprintf("* 新增字段: %s", name))

			// 添加类型和中文名称信息
			if newPropMap, ok := newProp.(map[string]interface{}); ok {
				if propType, ok := newPropMap["type"].(string); ok {
					builder.WriteString(fmt.Sprintf(" (%s)", propType))
				}

				// 添加中文名称信息
				if title, ok := newPropMap["title"].(string); ok && title != "" {
					builder.WriteString(fmt.Sprintf(" [%s]", title))
				}
			}

			// 添加必填信息
			if newRequiredFields[name] {
				builder.WriteString(" (必填)")
			}

			builder.WriteString("\n")
		}
	}

	// 最后显示字段修改
	if len(modifiedFields) > 0 {
		for _, field := range modifiedFields {
			// 显示字段名和中文名称（如果有）
			if field.newTitle != "" && field.newTitle != field.name {
				builder.WriteString(fmt.Sprintf("* 修改字段: %s [%s]\n", field.name, field.newTitle))
			} else {
				builder.WriteString(fmt.Sprintf("* 修改字段: %s\n", field.name))
			}

			// 显示类型变化（如果有）
			if field.oldType != field.newType && field.oldType != "" && field.newType != "" {
				builder.WriteString(fmt.Sprintf("  - 类型: %s -> %s\n", field.oldType, field.newType))
			}

			// 显示中文名称变化（如果有）
			if field.oldTitle != field.newTitle && field.oldTitle != "" && field.newTitle != "" {
				builder.WriteString(fmt.Sprintf("  - 名称: %s -> %s\n", field.oldTitle, field.newTitle))
			}

			// 显示说明变化（如果有）
			if field.oldDesc != field.newDesc && (field.oldDesc != "" || field.newDesc != "") {
				builder.WriteString(fmt.Sprintf("  - 说明: %s -> %s\n", field.oldDesc, field.newDesc))
			}

			// 显示必填状态变化（如果有）
			if field.hasRequiredChange {
				if field.newRequired {
					builder.WriteString("  - 变为必填\n")
				} else {
					builder.WriteString("  - 变为可选\n")
				}
			}

			// 显示其他属性变化
			for propName, change := range field.changes {
				// 跳过已单独处理的属性
				if propName == "type" || propName == "title" || propName == "description" {
					continue
				}

				// 格式化值更友好地显示
				oldValue := formatValue(change.old)
				newValue := formatValue(change.new)

				if oldValue != "" || newValue != "" {
					builder.WriteString(fmt.Sprintf("  - %s: %s -> %s\n", propName, oldValue, newValue))
				}
			}
		}
	}
}

// formatValue 将值格式化为字符串
func formatValue(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case bool:
		return fmt.Sprintf("%t", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case int:
		return fmt.Sprintf("%d", val)
	default:
		jsonStr, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(jsonStr)
	}
}

// contains 检查字符串切片是否包含特定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// interfaceSliceToStringSlice 将接口切片转换为字符串切片
func interfaceSliceToStringSlice(slice []interface{}) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

// equalStringSlices 比较两个字符串切片是否相等
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]bool)
	for _, item := range a {
		aMap[item] = true
	}

	for _, item := range b {
		if _, exists := aMap[item]; !exists {
			return false
		}
	}

	return true
}

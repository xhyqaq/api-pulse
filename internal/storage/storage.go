package storage

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/xhy/api-pulse/internal/apifox"
)

// ApiStore API 存储服务 - 纯内存实现
type ApiStore struct {
	apisByKey  map[string]apifox.StoredApiInfo // 使用 ApiKey 索引
	apisByPath map[string]apifox.StoredApiInfo // 使用 ApiPath 索引
	mutex      sync.RWMutex
	logger     *logrus.Logger
}

// NewApiStore 创建新的 API 存储服务
func NewApiStore(logger *logrus.Logger) *ApiStore {
	return &ApiStore{
		apisByKey:  make(map[string]apifox.StoredApiInfo),
		apisByPath: make(map[string]apifox.StoredApiInfo),
		logger:     logger,
	}
}

// SaveApi 保存 API 信息
func (s *ApiStore) SaveApi(apiInfo apifox.StoredApiInfo) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查是否存在旧的API信息
	oldApiInfo, exists := s.apisByKey[apiInfo.ApiKey]
	if exists {
		// 如果旧的API路径存在且与新的不同，需要删除旧的路径索引
		if oldApiInfo.ApiPath != "" && (oldApiInfo.Method != apiInfo.Method || oldApiInfo.ApiPath != apiInfo.ApiPath) {
			oldPathKey := fmt.Sprintf("%s %s", oldApiInfo.Method, oldApiInfo.ApiPath)
			delete(s.apisByPath, oldPathKey)
			s.logger.WithFields(logrus.Fields{
				"api_key":    apiInfo.ApiKey,
				"old_path":   oldPathKey,
				"new_method": apiInfo.Method,
				"new_path":   apiInfo.ApiPath,
			}).Debug("删除旧的API路径索引")
		}
	}

	// 更新Key索引
	s.apisByKey[apiInfo.ApiKey] = apiInfo

	// 如果 ApiPath 不为空，则也按路径索引
	if apiInfo.ApiPath != "" {
		pathKey := fmt.Sprintf("%s %s", apiInfo.Method, apiInfo.ApiPath)
		s.apisByPath[pathKey] = apiInfo
	}

	return nil
}

// GetApi 根据 ApiKey 获取 API 信息
func (s *ApiStore) GetApi(apiKey string) (apifox.StoredApiInfo, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	api, exists := s.apisByKey[apiKey]
	return api, exists
}

// GetApiByPath 根据 HTTP 方法和路径获取 API 信息
func (s *ApiStore) GetApiByPath(method, path string) (apifox.StoredApiInfo, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	pathKey := fmt.Sprintf("%s %s", method, path)
	api, exists := s.apisByPath[pathKey]
	return api, exists
}

// GetAllApis 获取所有 API 信息
func (s *ApiStore) GetAllApis() map[string]apifox.StoredApiInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 创建副本返回，避免原始数据被修改
	apis := make(map[string]apifox.StoredApiInfo, len(s.apisByKey))
	for k, v := range s.apisByKey {
		apis[k] = v
	}
	return apis
}

// GetAllApisByPath 获取所有按路径索引的 API 信息
func (s *ApiStore) GetAllApisByPath() map[string]apifox.StoredApiInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 创建副本返回，避免原始数据被修改
	apis := make(map[string]apifox.StoredApiInfo, len(s.apisByPath))
	for k, v := range s.apisByPath {
		apis[k] = v
	}
	return apis
}

// ClearAll 清空所有 API 信息
func (s *ApiStore) ClearAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.apisByKey = make(map[string]apifox.StoredApiInfo)
	s.apisByPath = make(map[string]apifox.StoredApiInfo)
}

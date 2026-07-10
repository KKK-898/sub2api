package service

import "context"

type apiKeyAuthCacheBatchInvalidator interface {
	DeleteAuthCachesAndPublish(ctx context.Context, cacheKeys []string) error
}

// InvalidateAuthCacheByKey 清除指定 API Key 的认证缓存
func (s *APIKeyService) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	if key == "" {
		return
	}
	cacheKey := s.authCacheKey(key)
	s.deleteAuthCache(ctx, cacheKey)
}

// InvalidateAuthCacheByUserID 清除用户相关的 API Key 认证缓存
func (s *APIKeyService) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
	if userID <= 0 {
		return
	}
	keys, err := s.apiKeyRepo.ListKeysByUserID(ctx, userID)
	if err != nil {
		return
	}
	s.deleteAuthCacheByKeys(ctx, keys)
}

// InvalidateAuthCacheByGroupID 清除分组相关的 API Key 认证缓存
func (s *APIKeyService) InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64) {
	if groupID <= 0 {
		return
	}
	keys, err := s.apiKeyRepo.ListKeysByGroupID(ctx, groupID)
	if err != nil {
		return
	}
	s.deleteAuthCacheByKeys(ctx, keys)
}

func (s *APIKeyService) deleteAuthCacheByKeys(ctx context.Context, keys []string) {
	if len(keys) == 0 {
		return
	}
	cacheKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		cacheKey := s.authCacheKey(key)
		cacheKeys = append(cacheKeys, cacheKey)
		if s.authCacheL1 != nil {
			s.authCacheL1.Del(cacheKey)
		}
	}
	if len(cacheKeys) == 0 || s.cache == nil {
		return
	}
	if batchCache, ok := s.cache.(apiKeyAuthCacheBatchInvalidator); ok {
		if err := batchCache.DeleteAuthCachesAndPublish(ctx, cacheKeys); err == nil {
			return
		}
	}
	for _, cacheKey := range cacheKeys {
		_ = s.cache.DeleteAuthCache(ctx, cacheKey)
		_ = s.cache.PublishAuthCacheInvalidation(ctx, cacheKey)
	}
}

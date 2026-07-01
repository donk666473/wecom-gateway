// Package debug 提供调试/诊断服务。
// 包括错误日志收集、平台状态统计、请求追踪等。
package debug

import (
	"sync"
	"time"
)

// ErrorRecord 错误记录条目。
type ErrorRecord struct {
	Time      time.Time `json:"time"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Caller    string    `json:"caller,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Stack     string    `json:"stack,omitempty"`
}

const defaultMaxRecords = 1000

var (
	records   []ErrorRecord
	mu        sync.RWMutex
	maxRecords = defaultMaxRecords
)

// Collect 收集一条错误记录（线程安全）。
func Collect(record ErrorRecord) {
	mu.Lock()
	defer mu.Unlock()
	records = append(records, record)
	if len(records) > maxRecords {
		records = records[len(records)-maxRecords:]
	}
}

// RecentErrors 返回最近的 limit 条错误记录。
// limit <= 0 时返回全部。
func RecentErrors(limit int) []ErrorRecord {
	mu.RLock()
	defer mu.RUnlock()

	if limit <= 0 || limit > len(records) {
		limit = len(records)
	}
	result := make([]ErrorRecord, limit)
	copy(result, records[len(records)-limit:])
	return result
}

// Stats 返回调试统计信息（平台、应用数、错误数等）。
func Stats() map[string]interface{} {
	mu.RLock()
	defer mu.RUnlock()

	return map[string]interface{}{
		"error_count": len(records),
	}
}

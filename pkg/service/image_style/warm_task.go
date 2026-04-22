/*
 * @Description: 缓存预热异步任务管理器（内存版）
 * @Author: 安知鱼
 *
 * 对应 Plan Phase 4 Task 4.5。功能职责：
 *   - 为 ImageStyleService.WarmCache 维护任务生命周期
 *   - 用 (policyID, styleName) 去重同一任务的并发触发
 *   - 对外暴露进度查询
 *
 * 设计要点：
 *   - 纯内存实现，无外部依赖（Plan §4.5 允许 Redis 或内存，MVP 选内存）
 *   - 任务完成后进度保留一段时间供前端轮询，超过保留期通过 Reap 手动回收
 *   - 任务 ID 使用 sha256(policyID|styleName|nowNS) 前 12 位 hex，避免对外泄露 PID
 */
package image_style

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ErrWarmTaskNotFound 查询不存在的预热任务时返回。
var ErrWarmTaskNotFound = errors.New("warm task not found")

// warmTaskManager 负责 WarmCache 任务的注册、更新与查询。
// 所有外部访问均加锁，内部状态通过 copy-on-read 避免调用方修改快照。
type warmTaskManager struct {
	mu         sync.RWMutex
	byTask     map[string]*WarmProgress
	activeKey  map[string]string // key => taskID（用于去重）
	now        func() time.Time  // 可注入便于测试
}

// newWarmTaskManager 构造管理器。now 可传 nil，默认使用 time.Now。
func newWarmTaskManager(now func() time.Time) *warmTaskManager {
	if now == nil {
		now = time.Now
	}
	return &warmTaskManager{
		byTask:    make(map[string]*WarmProgress),
		activeKey: make(map[string]string),
		now:       now,
	}
}

// warmTaskKey 构造 (policyID, styleName) 的去重键。
func warmTaskKey(policyID uint, styleName string) string {
	return strconv.FormatUint(uint64(policyID), 10) + ":" + styleName
}

// genTaskID 生成 sha256(policyID|styleName|unixNano|seq) 前 12 hex。
// 必须在 m.mu 已持锁（读写皆可）时调用，seq 由调用方传入以避免重入加锁。
func genTaskID(policyID uint, styleName string, nowNS int64, seq int) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%d|%d", policyID, styleName, nowNS, seq)
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// register 尝试登记新任务；若相同 (policyID, styleName) 已存在 running/pending
// 任务，则直接返回现有 taskID、ok=false，调用方应放弃启动新任务。
func (m *warmTaskManager) register(policyID uint, styleName string) (taskID string, ok bool) {
	key := warmTaskKey(policyID, styleName)

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, exists := m.activeKey[key]; exists {
		if prog, still := m.byTask[existing]; still && (prog.Status == "pending" || prog.Status == "running") {
			return existing, false
		}
	}

	taskID = genTaskID(policyID, styleName, m.now().UnixNano(), len(m.byTask))
	m.byTask[taskID] = &WarmProgress{
		TaskID:    taskID,
		PolicyID:  policyID,
		StyleName: styleName,
		Status:    "pending",
		StartedAt: m.now(),
	}
	m.activeKey[key] = taskID
	return taskID, true
}

// setTotal 写入任务的 Total；一旦已知总数就立即调用，UI 能即时显示进度条。
// 同步把状态从 pending 切到 running。
func (m *warmTaskManager) setTotal(taskID string, total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.byTask[taskID]; ok {
		p.Total = total
		p.Status = "running"
	}
}

// inc 累计 processed / failed 其中之一。lastErr 非空时覆盖 LastError。
func (m *warmTaskManager) inc(taskID string, kind string, lastErr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.byTask[taskID]
	if !ok {
		return
	}
	switch kind {
	case "processed":
		p.Processed++
	case "failed":
		p.Failed++
		if lastErr != "" {
			p.LastError = lastErr
		}
	}
}

// finish 结束任务并从 activeKey 中移除，保留 byTask 中的终态供查询。
// status 预期取值 "done" / "failed" / "cancelled"。
func (m *warmTaskManager) finish(taskID, status string, lastErr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.byTask[taskID]
	if !ok {
		return
	}
	p.Status = status
	p.FinishedAt = m.now()
	if lastErr != "" {
		p.LastError = lastErr
	}
	// 从 activeKey 删除，允许同策略+样式再次发起新任务
	for k, v := range m.activeKey {
		if v == taskID {
			delete(m.activeKey, k)
			break
		}
	}
}

// get 返回任务进度的拷贝；调用方可安全修改返回值而不影响内部状态。
func (m *warmTaskManager) get(taskID string) (*WarmProgress, error) {
	m.mu.RLock()
	p, ok := m.byTask[taskID]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrWarmTaskNotFound
	}
	copy := *p
	return &copy, nil
}

// reap 丢弃早于 cutoff 的已结束任务；由调用方决定何时调用。
// 管理器本身不启动后台 goroutine，避免与 Service 生命周期耦合。
func (m *warmTaskManager) reap(cutoff time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	for id, p := range m.byTask {
		if p.Status == "pending" || p.Status == "running" {
			continue
		}
		if !p.FinishedAt.IsZero() && p.FinishedAt.Before(cutoff) {
			delete(m.byTask, id)
			removed++
		}
	}
	return removed
}

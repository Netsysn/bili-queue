package queue

import (
	"fmt"
	"sync"
	"time"
)

type Status int

const (
	StatusActive     Status = iota // 排队中
	StatusInProgress               // 进行中（主播正在服务）
	StatusDone                     // 已完成
	StatusTimeout                  // 超时
)

type Entry struct {
	UID        int64     `json:"uid"`
	Username   string    `json:"username"`
	Avatar     string    `json:"avatar"`
	JoinedAt   time.Time `json:"joined_at"`
	Status     Status    `json:"status"`
	HelpType   string    `json:"help_type"`
	Server     string    `json:"server"`
	Message    string    `json:"message"`
	MedalName  string    `json:"medal_name"`
	MedalLevel int       `json:"medal_level"`
	UserLevel  int       `json:"user_level"`
}

type DanmakuLog struct {
	UID         int64     `json:"uid"`
	Username    string    `json:"username"`
	Avatar      string    `json:"avatar"`
	Content     string    `json:"content"`
	Time        time.Time `json:"time"`
	IsQueue     bool      `json:"is_queue"`
	FromCurrent bool      `json:"from_current"`
	HelpType    string    `json:"help_type,omitempty"`
	Server      string    `json:"server,omitempty"`
	MedalName   string    `json:"medal_name"`
	MedalLevel  int       `json:"medal_level"`
	UserLevel   int       `json:"user_level"`
	ReplyTo     string    `json:"reply_to"`
}

type Manager struct {
	mu             sync.Mutex
	items          []Entry
	uidSet         map[int64]int
	logs           []DanmakuLog
	onChange       func([]Entry, []DanmakuLog)
	timeoutMinutes int
	stopCh         chan struct{}
}

func New(onChange func([]Entry, []DanmakuLog), timeoutMinutes int) *Manager {
	if timeoutMinutes < 1 {
		timeoutMinutes = 5
	}
	return &Manager{
		items:          make([]Entry, 0),
		uidSet:         make(map[int64]int),
		logs:           make([]DanmakuLog, 0, 200),
		onChange:       onChange,
		timeoutMinutes: timeoutMinutes,
		stopCh:         make(chan struct{}),
	}
}

// AddLog 记录弹幕日志。每 3 条触发一次前端更新。
func (m *Manager) AddLog(log DanmakuLog) {
	m.mu.Lock()
	m.logs = append(m.logs, log)
	if len(m.logs) > 200 {
		m.logs = m.logs[len(m.logs)-200:]
	}
	m.mu.Unlock()
	m.emitSafe()
}

func (m *Manager) Enqueue(uid int64, username, avatar, helpType, server, message, medalName string, medalLevel, userLevel int) bool {
	m.mu.Lock()
	if _, ok := m.uidSet[uid]; ok {
		m.mu.Unlock()
		return false
	}
	entry := Entry{
		UID: uid, Username: username, Avatar: avatar,
		JoinedAt: time.Now(), Status: StatusActive,
		HelpType: helpType, Server: server, Message: message,
		MedalName: medalName, MedalLevel: medalLevel, UserLevel: userLevel,
	}
	m.items = append(m.items, entry)
	m.uidSet[uid] = len(m.items) - 1
	m.mu.Unlock()
	m.emitSafe()
	return true
}

func (m *Manager) Complete() {
	m.mu.Lock()
	for i := range m.items {
		if m.items[i].Status == StatusInProgress {
			uid := m.items[i].UID
			delete(m.uidSet, uid)
			m.items = append(m.items[:i], m.items[i+1:]...)
			for j := i; j < len(m.items); j++ {
				m.uidSet[m.items[j].UID] = j
			}
			break
		}
	}
	m.mu.Unlock()
	m.emitSafe()
}

// Start 将第一个排队中的人标为进行中。返回错误如果已有人进行中。
func (m *Manager) Start() error {
	m.mu.Lock()
	hasInProgress := false
	for _, it := range m.items {
		if it.Status == StatusInProgress {
			hasInProgress = true
			break
		}
	}
	if hasInProgress {
		m.mu.Unlock()
		return fmt.Errorf("请先完成当前进行中的任务")
	}
	found := false
	for i := range m.items {
		if m.items[i].Status == StatusActive {
			m.items[i].Status = StatusInProgress
			m.items[i].JoinedAt = time.Now()
			found = true
			break
		}
	}
	m.mu.Unlock()
	if found {
		m.emitSafe()
		return nil
	}
	return fmt.Errorf("没有排队中的人")
}

func (m *Manager) Skip() {
	m.mu.Lock()
	idx := -1
	for i := range m.items {
		if m.items[i].Status == StatusActive {
			idx = i
			break
		}
	}
	if idx >= 0 && idx < len(m.items)-1 {
		m.items[idx], m.items[idx+1] = m.items[idx+1], m.items[idx]
		m.uidSet[m.items[idx].UID] = idx
		m.uidSet[m.items[idx+1].UID] = idx + 1
	}
	m.mu.Unlock()
	m.emitSafe()
}

func (m *Manager) Remove(index int) {
	m.mu.Lock()
	if index >= 0 && index < len(m.items) {
		delete(m.uidSet, m.items[index].UID)
		m.items = append(m.items[:index], m.items[index+1:]...)
		for i := index; i < len(m.items); i++ {
			m.uidSet[m.items[i].UID] = i
		}
	}
	m.mu.Unlock()
	m.emitSafe()
}

func (m *Manager) Restore(index int) {
	m.mu.Lock()
	if index >= 0 && index < len(m.items) && m.items[index].Status == StatusTimeout {
		m.items[index].Status = StatusActive
	}
	m.mu.Unlock()
	m.emitSafe()
}

func (m *Manager) List() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Entry, len(m.items))
	copy(out, m.items)
	return out
}

func (m *Manager) Logs() []DanmakuLog {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]DanmakuLog, len(m.logs))
	copy(out, m.logs)
	return out
}

func (m *Manager) StartTimeoutChecker() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.checkTimeout()
			case <-m.stopCh:
				return
			}
		}
	}()
}

func (m *Manager) Stop() { close(m.stopCh) }

func (m *Manager) checkTimeout() {
	m.mu.Lock()
	changed := false
	threshold := time.Duration(m.timeoutMinutes) * time.Minute
	for i := range m.items {
		if m.items[i].Status == StatusInProgress && time.Since(m.items[i].JoinedAt) > threshold {
			m.items[i].Status = StatusTimeout
			changed = true
		}
	}
	m.mu.Unlock()
	if changed {
		m.emitSafe()
	}
}

func (m *Manager) emitSafe() {
	if m.onChange == nil {
		return
	}
	m.mu.Lock()
	out := make([]Entry, len(m.items))
	copy(out, m.items)
	logs := make([]DanmakuLog, len(m.logs))
	copy(logs, m.logs)
	m.mu.Unlock()
	m.onChange(out, logs)
}

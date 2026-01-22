package db

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/gotunnel/pkg/protocol"
)

// SQLiteStore SQLite 存储实现
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore 创建 SQLite 存储
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	s := &SQLiteStore{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// init 初始化数据库表
func (s *SQLiteStore) init() error {
	// 创建客户端表
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS clients (
			id TEXT PRIMARY KEY,
			nickname TEXT NOT NULL DEFAULT '',
			rules TEXT NOT NULL DEFAULT '[]',
			plugins TEXT NOT NULL DEFAULT '[]'
		)
	`)
	if err != nil {
		return err
	}

	// 迁移：添加 nickname 列
	s.db.Exec(`ALTER TABLE clients ADD COLUMN nickname TEXT NOT NULL DEFAULT ''`)
	// 迁移：添加 plugins 列
	s.db.Exec(`ALTER TABLE clients ADD COLUMN plugins TEXT NOT NULL DEFAULT '[]'`)

	// 创建 JS 插件表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS js_plugins (
			name TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			signature TEXT NOT NULL DEFAULT '',
			description TEXT,
			author TEXT,
			version TEXT DEFAULT '',
			auto_push TEXT NOT NULL DEFAULT '[]',
			config TEXT NOT NULL DEFAULT '{}',
			auto_start INTEGER DEFAULT 1,
			enabled INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 迁移：添加 signature 列
	s.db.Exec(`ALTER TABLE js_plugins ADD COLUMN signature TEXT NOT NULL DEFAULT ''`)
	// 迁移：添加 version 列
	s.db.Exec(`ALTER TABLE js_plugins ADD COLUMN version TEXT DEFAULT ''`)
	// 迁移：添加 updated_at 列
	s.db.Exec(`ALTER TABLE js_plugins ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP`)

	// 创建流量统计表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS traffic_stats (
			hour_ts INTEGER PRIMARY KEY,
			inbound INTEGER NOT NULL DEFAULT 0,
			outbound INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}

	// 创建总流量表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS traffic_total (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			inbound INTEGER NOT NULL DEFAULT 0,
			outbound INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}

	// 初始化总流量记录
	s.db.Exec(`INSERT OR IGNORE INTO traffic_total (id, inbound, outbound) VALUES (1, 0, 0)`)

	return nil
}

// Close 关闭数据库连接
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// GetAllClients 获取所有客户端
func (s *SQLiteStore) GetAllClients() ([]Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, nickname, rules, plugins FROM clients`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var c Client
		var rulesJSON, pluginsJSON string
		if err := rows.Scan(&c.ID, &c.Nickname, &rulesJSON, &pluginsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rulesJSON), &c.Rules); err != nil {
			c.Rules = []protocol.ProxyRule{}
		}
		if err := json.Unmarshal([]byte(pluginsJSON), &c.Plugins); err != nil {
			c.Plugins = []ClientPlugin{}
		}
		clients = append(clients, c)
	}
	return clients, nil
}

// GetClient 获取单个客户端
func (s *SQLiteStore) GetClient(id string) (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var c Client
	var rulesJSON, pluginsJSON string
	err := s.db.QueryRow(`SELECT id, nickname, rules, plugins FROM clients WHERE id = ?`, id).Scan(&c.ID, &c.Nickname, &rulesJSON, &pluginsJSON)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(rulesJSON), &c.Rules); err != nil {
		c.Rules = []protocol.ProxyRule{}
	}
	if err := json.Unmarshal([]byte(pluginsJSON), &c.Plugins); err != nil {
		c.Plugins = []ClientPlugin{}
	}
	return &c, nil
}

// CreateClient 创建客户端
func (s *SQLiteStore) CreateClient(c *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rulesJSON, err := json.Marshal(c.Rules)
	if err != nil {
		return err
	}
	pluginsJSON, err := json.Marshal(c.Plugins)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO clients (id, nickname, rules, plugins) VALUES (?, ?, ?, ?)`,
		c.ID, c.Nickname, string(rulesJSON), string(pluginsJSON))
	return err
}

// UpdateClient 更新客户端
func (s *SQLiteStore) UpdateClient(c *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rulesJSON, err := json.Marshal(c.Rules)
	if err != nil {
		return err
	}
	pluginsJSON, err := json.Marshal(c.Plugins)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE clients SET nickname = ?, rules = ?, plugins = ? WHERE id = ?`,
		c.Nickname, string(rulesJSON), string(pluginsJSON), c.ID)
	return err
}

// DeleteClient 删除客户端
func (s *SQLiteStore) DeleteClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM clients WHERE id = ?`, id)
	return err
}

// ClientExists 检查客户端是否存在
func (s *SQLiteStore) ClientExists(id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM clients WHERE id = ?`, id).Scan(&count)
	return count > 0, err
}

// GetClientRules 获取客户端规则
func (s *SQLiteStore) GetClientRules(id string) ([]protocol.ProxyRule, error) {
	c, err := s.GetClient(id)
	if err != nil {
		return nil, err
	}
	return c.Rules, nil
}

// ========== JS 插件存储方法 ==========

// GetAllJSPlugins 获取所有 JS 插件
func (s *SQLiteStore) GetAllJSPlugins() ([]JSPlugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT name, source, signature, description, author, version, auto_push, config, auto_start, enabled
		FROM js_plugins
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []JSPlugin
	for rows.Next() {
		var p JSPlugin
		var autoPushJSON, configJSON string
		var version sql.NullString
		var autoStart, enabled int
		err := rows.Scan(&p.Name, &p.Source, &p.Signature, &p.Description, &p.Author,
			&version, &autoPushJSON, &configJSON, &autoStart, &enabled)
		if err != nil {
			return nil, err
		}
		p.Version = version.String
		json.Unmarshal([]byte(autoPushJSON), &p.AutoPush)
		json.Unmarshal([]byte(configJSON), &p.Config)
		p.AutoStart = autoStart == 1
		p.Enabled = enabled == 1
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// GetJSPlugin 获取单个 JS 插件
func (s *SQLiteStore) GetJSPlugin(name string) (*JSPlugin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var p JSPlugin
	var autoPushJSON, configJSON string
	var version sql.NullString
	var autoStart, enabled int
	err := s.db.QueryRow(`
		SELECT name, source, signature, description, author, version, auto_push, config, auto_start, enabled
		FROM js_plugins WHERE name = ?
	`, name).Scan(&p.Name, &p.Source, &p.Signature, &p.Description, &p.Author,
		&version, &autoPushJSON, &configJSON, &autoStart, &enabled)
	if err != nil {
		return nil, err
	}
	p.Version = version.String
	json.Unmarshal([]byte(autoPushJSON), &p.AutoPush)
	json.Unmarshal([]byte(configJSON), &p.Config)
	p.AutoStart = autoStart == 1
	p.Enabled = enabled == 1
	return &p, nil
}

// SaveJSPlugin 保存 JS 插件
func (s *SQLiteStore) SaveJSPlugin(p *JSPlugin) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	autoPushJSON, _ := json.Marshal(p.AutoPush)
	configJSON, _ := json.Marshal(p.Config)
	autoStart, enabled := 0, 0
	if p.AutoStart {
		autoStart = 1
	}
	if p.Enabled {
		enabled = 1
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO js_plugins
		(name, source, signature, description, author, version, auto_push, config, auto_start, enabled, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, p.Name, p.Source, p.Signature, p.Description, p.Author, p.Version,
		string(autoPushJSON), string(configJSON), autoStart, enabled)
	return err
}

// DeleteJSPlugin 删除 JS 插件
func (s *SQLiteStore) DeleteJSPlugin(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM js_plugins WHERE name = ?`, name)
	return err
}

// SetJSPluginEnabled 设置 JS 插件启用状态
func (s *SQLiteStore) SetJSPluginEnabled(name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec(`UPDATE js_plugins SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`, val, name)
	return err
}

// UpdateJSPluginConfig 更新 JS 插件配置
func (s *SQLiteStore) UpdateJSPluginConfig(name string, config map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	configJSON, _ := json.Marshal(config)
	_, err := s.db.Exec(`UPDATE js_plugins SET config = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`, string(configJSON), name)
	return err
}

// ========== 流量统计方法 ==========

// getHourTimestamp 获取当前小时的时间戳
func getHourTimestamp() int64 {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location()).Unix()
}

// AddTraffic 添加流量记录
func (s *SQLiteStore) AddTraffic(inbound, outbound int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hourTs := getHourTimestamp()

	// 更新小时统计
	_, err := s.db.Exec(`
		INSERT INTO traffic_stats (hour_ts, inbound, outbound) VALUES (?, ?, ?)
		ON CONFLICT(hour_ts) DO UPDATE SET inbound = inbound + ?, outbound = outbound + ?
	`, hourTs, inbound, outbound, inbound, outbound)
	if err != nil {
		return err
	}

	// 更新总流量
	_, err = s.db.Exec(`
		UPDATE traffic_total SET inbound = inbound + ?, outbound = outbound + ? WHERE id = 1
	`, inbound, outbound)
	return err
}

// GetTotalTraffic 获取总流量
func (s *SQLiteStore) GetTotalTraffic() (inbound, outbound int64, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	err = s.db.QueryRow(`SELECT inbound, outbound FROM traffic_total WHERE id = 1`).Scan(&inbound, &outbound)
	return
}

// Get24HourTraffic 获取24小时流量
func (s *SQLiteStore) Get24HourTraffic() (inbound, outbound int64, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	err = s.db.QueryRow(`
		SELECT COALESCE(SUM(inbound), 0), COALESCE(SUM(outbound), 0) 
		FROM traffic_stats WHERE hour_ts >= ?
	`, cutoff).Scan(&inbound, &outbound)
	return
}

// GetHourlyTraffic 获取每小时流量记录
func (s *SQLiteStore) GetHourlyTraffic(hours int) ([]TrafficRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	rows, err := s.db.Query(`
		SELECT hour_ts, inbound, outbound FROM traffic_stats 
		WHERE hour_ts >= ? ORDER BY hour_ts ASC
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TrafficRecord
	for rows.Next() {
		var r TrafficRecord
		if err := rows.Scan(&r.Timestamp, &r.Inbound, &r.Outbound); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

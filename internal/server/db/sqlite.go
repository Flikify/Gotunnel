package db

import (
	"database/sql"
	"encoding/json"
	"sync"

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

	// 创建插件表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS plugins (
			name TEXT PRIMARY KEY,
			version TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'proxy',
			source TEXT NOT NULL DEFAULT 'wasm',
			description TEXT,
			author TEXT,
			icon TEXT,
			checksum TEXT,
			size INTEGER DEFAULT 0,
			enabled INTEGER DEFAULT 1,
			wasm_data BLOB
		)
	`)
	if err != nil {
		return err
	}

	// 迁移：添加 icon 列
	s.db.Exec(`ALTER TABLE plugins ADD COLUMN icon TEXT`)

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

// ========== 插件存储方法 ==========

// GetAllPlugins 获取所有插件
func (s *SQLiteStore) GetAllPlugins() ([]PluginData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT name, version, type, source, description, author, icon, checksum, size, enabled
		FROM plugins
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []PluginData
	for rows.Next() {
		var p PluginData
		var enabled int
		var icon sql.NullString
		err := rows.Scan(&p.Name, &p.Version, &p.Type, &p.Source,
			&p.Description, &p.Author, &icon, &p.Checksum, &p.Size, &enabled)
		if err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		p.Icon = icon.String
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// GetPlugin 获取单个插件
func (s *SQLiteStore) GetPlugin(name string) (*PluginData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var p PluginData
	var enabled int
	var icon sql.NullString
	err := s.db.QueryRow(`
		SELECT name, version, type, source, description, author, icon, checksum, size, enabled
		FROM plugins WHERE name = ?
	`, name).Scan(&p.Name, &p.Version, &p.Type, &p.Source,
		&p.Description, &p.Author, &icon, &p.Checksum, &p.Size, &enabled)
	if err != nil {
		return nil, err
	}
	p.Enabled = enabled == 1
	p.Icon = icon.String
	return &p, nil
}

// SavePlugin 保存插件
func (s *SQLiteStore) SavePlugin(p *PluginData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	enabled := 0
	if p.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO plugins
		(name, version, type, source, description, author, icon, checksum, size, enabled, wasm_data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.Name, p.Version, p.Type, p.Source, p.Description, p.Author,
		p.Icon, p.Checksum, p.Size, enabled, p.WASMData)
	return err
}

// DeletePlugin 删除插件
func (s *SQLiteStore) DeletePlugin(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM plugins WHERE name = ?`, name)
	return err
}

// SetPluginEnabled 设置插件启用状态
func (s *SQLiteStore) SetPluginEnabled(name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec(`UPDATE plugins SET enabled = ? WHERE name = ?`, val, name)
	return err
}

// GetPluginWASM 获取插件 WASM 数据
func (s *SQLiteStore) GetPluginWASM(name string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var data []byte
	err := s.db.QueryRow(`SELECT wasm_data FROM plugins WHERE name = ?`, name).Scan(&data)
	return data, err
}

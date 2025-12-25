package db

import (
	"database/sql"
	"encoding/json"
	"sync"

	_ "github.com/mattn/go-sqlite3"

	"github.com/gotunnel/pkg/protocol"
)

// SQLiteStore SQLite 存储实现
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore 创建 SQLite 存储
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
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
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS clients (
			id TEXT PRIMARY KEY,
			rules TEXT NOT NULL DEFAULT '[]'
		);
	`)
	return err
}

// Close 关闭数据库连接
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// GetAllClients 获取所有客户端
func (s *SQLiteStore) GetAllClients() ([]Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, rules FROM clients`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var c Client
		var rulesJSON string
		if err := rows.Scan(&c.ID, &rulesJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rulesJSON), &c.Rules); err != nil {
			c.Rules = []protocol.ProxyRule{}
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
	var rulesJSON string
	err := s.db.QueryRow(`SELECT id, rules FROM clients WHERE id = ?`, id).Scan(&c.ID, &rulesJSON)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(rulesJSON), &c.Rules); err != nil {
		c.Rules = []protocol.ProxyRule{}
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
	_, err = s.db.Exec(`INSERT INTO clients (id, rules) VALUES (?, ?)`, c.ID, string(rulesJSON))
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
	_, err = s.db.Exec(`UPDATE clients SET rules = ? WHERE id = ?`, string(rulesJSON), c.ID)
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

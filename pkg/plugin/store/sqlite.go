package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gotunnel/pkg/plugin"
	_ "modernc.org/sqlite"
)

// SQLiteStore SQLite 实现的 PluginStore
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore 创建 SQLite plugin 存储
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// init 初始化数据库表
func (s *SQLiteStore) init() error {
	query := `
	CREATE TABLE IF NOT EXISTS plugins (
		name TEXT PRIMARY KEY,
		version TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'proxy',
		source TEXT NOT NULL DEFAULT 'wasm',
		description TEXT,
		author TEXT,
		checksum TEXT NOT NULL,
		size INTEGER NOT NULL,
		capabilities TEXT NOT NULL DEFAULT '[]',
		config_schema TEXT NOT NULL DEFAULT '{}',
		wasm_data BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := s.db.Exec(query)
	return err
}

// GetAllPlugins 返回所有存储的 plugins
func (s *SQLiteStore) GetAllPlugins() ([]plugin.PluginMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT name, version, type, source, description, author,
		       checksum, size, capabilities, config_schema
		FROM plugins`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []plugin.PluginMetadata
	for rows.Next() {
		var m plugin.PluginMetadata
		var capJSON, configJSON string
		err := rows.Scan(&m.Name, &m.Version, &m.Type, &m.Source,
			&m.Description, &m.Author, &m.Checksum, &m.Size,
			&capJSON, &configJSON)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(capJSON), &m.Capabilities)
		json.Unmarshal([]byte(configJSON), &m.ConfigSchema)
		plugins = append(plugins, m)
	}
	return plugins, rows.Err()
}

// GetPlugin 返回指定 plugin 的元数据
func (s *SQLiteStore) GetPlugin(name string) (*plugin.PluginMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m plugin.PluginMetadata
	var capJSON, configJSON string
	err := s.db.QueryRow(`
		SELECT name, version, type, source, description, author,
		       checksum, size, capabilities, config_schema
		FROM plugins WHERE name = ?`, name).Scan(
		&m.Name, &m.Version, &m.Type, &m.Source,
		&m.Description, &m.Author, &m.Checksum, &m.Size,
		&capJSON, &configJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(capJSON), &m.Capabilities)
	json.Unmarshal([]byte(configJSON), &m.ConfigSchema)
	return &m, nil
}

// GetPluginData 返回 WASM 二进制
func (s *SQLiteStore) GetPluginData(name string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	err := s.db.QueryRow(`SELECT wasm_data FROM plugins WHERE name = ?`, name).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return data, err
}

// SavePlugin 存储 plugin
func (s *SQLiteStore) SavePlugin(metadata plugin.PluginMetadata, wasmData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	capJSON, _ := json.Marshal(metadata.Capabilities)
	configJSON, _ := json.Marshal(metadata.ConfigSchema)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO plugins
		(name, version, type, source, description, author, checksum, size,
		 capabilities, config_schema, wasm_data, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		metadata.Name, metadata.Version, metadata.Type, metadata.Source,
		metadata.Description, metadata.Author, metadata.Checksum, metadata.Size,
		string(capJSON), string(configJSON), wasmData, time.Now())
	return err
}

// DeletePlugin 删除 plugin
func (s *SQLiteStore) DeletePlugin(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM plugins WHERE name = ?`, name)
	return err
}

// PluginExists 检查 plugin 是否存在
func (s *SQLiteStore) PluginExists(name string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM plugins WHERE name = ?`, name).Scan(&count)
	return count > 0, err
}

// Close 关闭存储
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

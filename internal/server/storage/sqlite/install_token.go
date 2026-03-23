package sqlite

// CreateInstallToken 创建安装token
func (s *SQLiteStore) CreateInstallToken(token *InstallToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`INSERT INTO install_tokens (token, client_id, created_at, used) VALUES (?, '', ?, ?)`,
		token.Token, token.CreatedAt, 0)
	return err
}

// GetInstallToken 获取安装token
func (s *SQLiteStore) GetInstallToken(token string) (*InstallToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var t InstallToken
	var used int
	err := s.db.QueryRow(`SELECT token, created_at, used FROM install_tokens WHERE token = ?`, token).
		Scan(&t.Token, &t.CreatedAt, &used)
	if err != nil {
		return nil, err
	}
	t.Used = used == 1
	return &t, nil
}

// MarkTokenUsed 标记token已使用
func (s *SQLiteStore) MarkTokenUsed(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE install_tokens SET used = 1 WHERE token = ?`, token)
	return err
}

// DeleteExpiredTokens 删除过期token
func (s *SQLiteStore) DeleteExpiredTokens(expireTime int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM install_tokens WHERE created_at < ?`, expireTime)
	return err
}

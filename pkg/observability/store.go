package observability

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultQueryLimit = 100
	maxQueryLimit     = 1000
)

type StoreOptions struct {
	RootDir       string
	RetentionDays int
	NodeID        string
	NodeRole      string
}

type SegmentManifest struct {
	MinTs      int64    `json:"min_ts"`
	MaxTs      int64    `json:"max_ts"`
	Count      int      `json:"count"`
	Levels     []string `json:"levels,omitempty"`
	Components []string `json:"components,omitempty"`
	Size       int64    `json:"size"`
}

type DiagnosticStore struct {
	rootDir       string
	retentionDays int
	nodeID        string
	nodeRole      string

	mu          sync.RWMutex
	subscribers map[int]subscriber
	nextSubID   int
	tail        []DiagnosticRecord
	maxTail     int
}

type subscriber struct {
	query LogQuery
	ch    chan DiagnosticRecord
}

func NewDiagnosticStore(opts StoreOptions) (*DiagnosticStore, error) {
	if opts.RootDir == "" {
		return nil, errors.New("diagnostic store root dir is required")
	}
	if opts.RetentionDays <= 0 {
		opts.RetentionDays = 7
	}
	if err := os.MkdirAll(opts.RootDir, 0755); err != nil {
		return nil, err
	}
	store := &DiagnosticStore{
		rootDir:       opts.RootDir,
		retentionDays: opts.RetentionDays,
		nodeID:        opts.NodeID,
		nodeRole:      opts.NodeRole,
		subscribers:   make(map[int]subscriber),
		maxTail:       512,
	}
	if err := store.cleanupExpired(time.Now()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *DiagnosticStore) Record(record DiagnosticRecord) error {
	record = record.Normalize(time.Now())
	if record.NodeID == "" {
		record.NodeID = s.nodeID
	}
	if record.NodeRole == "" {
		record.NodeRole = s.nodeRole
	}

	line, err := json.Marshal(record)
	if err != nil {
		return err
	}

	ts := time.UnixMilli(record.Timestamp)
	dir := filepath.Join(s.rootDir, ts.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	segmentPath := filepath.Join(dir, ts.Format("15")+".ndjson")
	f, err := os.OpenFile(segmentPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := s.updateManifest(segmentPath, record); err != nil {
		return err
	}

	s.mu.Lock()
	s.tail = append(s.tail, record)
	if len(s.tail) > s.maxTail {
		s.tail = s.tail[len(s.tail)-s.maxTail:]
	}
	subs := make([]subscriber, 0, len(s.subscribers))
	for _, sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.mu.Unlock()

	for _, sub := range subs {
		if !matchesQuery(record, sub.query) {
			continue
		}
		select {
		case sub.ch <- record:
		default:
		}
	}

	return nil
}

func (s *DiagnosticStore) Tail(limit int) []DiagnosticRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.tail) {
		limit = len(s.tail)
	}
	out := make([]DiagnosticRecord, limit)
	copy(out, s.tail[len(s.tail)-limit:])
	return out
}

func (s *DiagnosticStore) Query(query LogQuery) (LogPage, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	cursor, err := decodeCursor(query.Cursor)
	if err != nil {
		return LogPage{}, err
	}

	records, err := s.scan(query)
	if err != nil {
		return LogPage{}, err
	}
	if cursor >= len(records) {
		return LogPage{EOF: true}, nil
	}

	end := cursor + limit
	if end > len(records) {
		end = len(records)
	}

	page := LogPage{
		Records: append([]DiagnosticRecord(nil), records[cursor:end]...),
		EOF:     end >= len(records),
	}
	if !page.EOF {
		page.NextCursor = encodeCursor(end)
	}
	return page, nil
}

func (s *DiagnosticStore) Follow(query LogQuery) (<-chan DiagnosticRecord, func(), error) {
	ch := make(chan DiagnosticRecord, 128)
	s.mu.Lock()
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = subscriber{query: query, ch: ch}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		sub, ok := s.subscribers[id]
		if ok {
			delete(s.subscribers, id)
		}
		s.mu.Unlock()
		if ok {
			close(sub.ch)
		}
	}
	return ch, cancel, nil
}

func (s *DiagnosticStore) RootDir() string {
	return s.rootDir
}

func (s *DiagnosticStore) scan(query LogQuery) ([]DiagnosticRecord, error) {
	files, err := s.segmentFiles(query)
	if err != nil {
		return nil, err
	}
	records := make([]DiagnosticRecord, 0, 128)
	for _, path := range files {
		part, err := readSegment(path, query)
		if err != nil {
			continue
		}
		records = append(records, part...)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp < records[j].Timestamp
	})
	return records, nil
}

func (s *DiagnosticStore) segmentFiles(query LogQuery) ([]string, error) {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, day := range entries {
		if !day.IsDir() {
			continue
		}
		dayPath := filepath.Join(s.rootDir, day.Name())
		hourEntries, err := os.ReadDir(dayPath)
		if err != nil {
			continue
		}
		for _, entry := range hourEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ndjson") {
				continue
			}
			path := filepath.Join(dayPath, entry.Name())
			if !segmentMayMatch(path, query) {
				continue
			}
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files, nil
}

func segmentMayMatch(segmentPath string, query LogQuery) bool {
	manifestPath := strings.TrimSuffix(segmentPath, ".ndjson") + ".manifest.json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return true
	}
	var manifest SegmentManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return true
	}
	if query.TimeFrom > 0 && manifest.MaxTs > 0 && manifest.MaxTs < query.TimeFrom {
		return false
	}
	if query.TimeTo > 0 && manifest.MinTs > 0 && manifest.MinTs > query.TimeTo {
		return false
	}
	if query.Level != "" && !contains(manifest.Levels, query.Level) {
		return false
	}
	if query.Component != "" && !contains(manifest.Components, query.Component) {
		return false
	}
	return true
}

func readSegment(segmentPath string, query LogQuery) ([]DiagnosticRecord, error) {
	f, err := os.Open(segmentPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var records []DiagnosticRecord
	for scanner.Scan() {
		var record DiagnosticRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if !matchesQuery(record, query) {
			continue
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

func matchesQuery(record DiagnosticRecord, query LogQuery) bool {
	if query.TimeFrom > 0 && record.Timestamp < query.TimeFrom {
		return false
	}
	if query.TimeTo > 0 && record.Timestamp > query.TimeTo {
		return false
	}
	if query.Level != "" && record.Level != query.Level {
		return false
	}
	if query.Component != "" && record.Component != query.Component {
		return false
	}
	if query.EventCodePrefix != "" && !strings.HasPrefix(record.EventCode, query.EventCodePrefix) {
		return false
	}
	if query.TextContains != "" {
		text := strings.ToLower(query.TextContains)
		if !strings.Contains(strings.ToLower(record.Message), text) {
			matched := false
			for key, value := range record.Fields {
				if strings.Contains(strings.ToLower(key), text) || strings.Contains(strings.ToLower(value), text) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
	}
	return true
}

func (s *DiagnosticStore) updateManifest(segmentPath string, record DiagnosticRecord) error {
	manifestPath := strings.TrimSuffix(segmentPath, ".ndjson") + ".manifest.json"
	var manifest SegmentManifest
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		_ = json.Unmarshal(data, &manifest)
	}
	if manifest.MinTs == 0 || record.Timestamp < manifest.MinTs {
		manifest.MinTs = record.Timestamp
	}
	if record.Timestamp > manifest.MaxTs {
		manifest.MaxTs = record.Timestamp
	}
	manifest.Count++
	manifest.Levels = appendUnique(manifest.Levels, record.Level)
	manifest.Components = appendUnique(manifest.Components, record.Component)
	if stat, err := os.Stat(segmentPath); err == nil {
		manifest.Size = stat.Size()
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, payload, 0644)
}

func (s *DiagnosticStore) cleanupExpired(now time.Time) error {
	cutoff := now.AddDate(0, 0, -s.retentionDays)
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		day, err := time.Parse("2006-01-02", entry.Name())
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(s.rootDir, entry.Name()))
		}
	}
	return nil
}

func appendUnique(values []string, value string) []string {
	if value == "" || contains(values, value) {
		return values
	}
	return append(values, value)
}

func contains(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

func encodeCursor(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	payload, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("decode cursor: %w", err)
	}
	offset, err := strconv.Atoi(string(payload))
	if err != nil {
		return 0, fmt.Errorf("parse cursor: %w", err)
	}
	if offset < 0 {
		return 0, errors.New("cursor must be non-negative")
	}
	return offset, nil
}

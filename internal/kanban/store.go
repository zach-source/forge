package kanban

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store provides persistence for kanban issues.
type Store struct {
	db *sql.DB
}

// NewStore creates a new store with the given database path.
// Use ":memory:" for an in-memory database.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return store, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates the database schema.
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS issues (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL DEFAULT 'backlog',
		priority TEXT NOT NULL DEFAULT 'medium',
		labels TEXT, -- JSON array
		assignee TEXT,
		parent_id TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES issues(id) ON DELETE SET NULL
	);

	CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(status);
	CREATE INDEX IF NOT EXISTS idx_issues_priority ON issues(priority);
	CREATE INDEX IF NOT EXISTS idx_issues_parent ON issues(parent_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Create inserts a new issue.
func (s *Store) Create(issue *Issue) error {
	if issue.ID == "" {
		issue.ID = generateID()
	}
	now := time.Now()
	issue.CreatedAt = now
	issue.UpdatedAt = now

	if issue.Status == "" {
		issue.Status = StatusBacklog
	}
	if issue.Priority == "" {
		issue.Priority = PriorityMedium
	}

	labels, err := json.Marshal(issue.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO issues (id, title, description, status, priority, labels, assignee, parent_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, issue.ID, issue.Title, issue.Description, issue.Status, issue.Priority,
		string(labels), issue.Assignee, nullString(issue.ParentID),
		issue.CreatedAt, issue.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting issue: %w", err)
	}
	return nil
}

// Get retrieves an issue by ID.
func (s *Store) Get(id string) (*Issue, error) {
	row := s.db.QueryRow(`
		SELECT id, title, description, status, priority, labels, assignee, parent_id, created_at, updated_at
		FROM issues WHERE id = ?
	`, id)

	return scanIssue(row)
}

// Update updates an existing issue.
func (s *Store) Update(issue *Issue) error {
	issue.UpdatedAt = time.Now()

	labels, err := json.Marshal(issue.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE issues SET
			title = ?, description = ?, status = ?, priority = ?,
			labels = ?, assignee = ?, parent_id = ?, updated_at = ?
		WHERE id = ?
	`, issue.Title, issue.Description, issue.Status, issue.Priority,
		string(labels), issue.Assignee, nullString(issue.ParentID),
		issue.UpdatedAt, issue.ID)

	if err != nil {
		return fmt.Errorf("updating issue: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("issue not found: %s", issue.ID)
	}
	return nil
}

// Delete removes an issue by ID.
func (s *Store) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM issues WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting issue: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("issue not found: %s", id)
	}
	return nil
}

// List retrieves all issues, optionally filtered by status.
func (s *Store) List(status ...Status) ([]*Issue, error) {
	var rows *sql.Rows
	var err error

	if len(status) == 0 {
		rows, err = s.db.Query(`
			SELECT id, title, description, status, priority, labels, assignee, parent_id, created_at, updated_at
			FROM issues ORDER BY created_at DESC
		`)
	} else {
		// Build IN clause
		args := make([]any, len(status))
		placeholders := ""
		for i, s := range status {
			args[i] = string(s)
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
		}
		rows, err = s.db.Query(fmt.Sprintf(`
			SELECT id, title, description, status, priority, labels, assignee, parent_id, created_at, updated_at
			FROM issues WHERE status IN (%s) ORDER BY created_at DESC
		`, placeholders), args...)
	}

	if err != nil {
		return nil, fmt.Errorf("querying issues: %w", err)
	}
	defer rows.Close()

	var issues []*Issue
	for rows.Next() {
		issue, err := scanIssueRows(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

// Move changes an issue's status.
func (s *Store) Move(id string, status Status) error {
	result, err := s.db.Exec(`
		UPDATE issues SET status = ?, updated_at = ? WHERE id = ?
	`, status, time.Now(), id)

	if err != nil {
		return fmt.Errorf("moving issue: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("issue not found: %s", id)
	}
	return nil
}

// GetBoard returns issues organized by status columns.
func (s *Store) GetBoard() (*Board, error) {
	issues, err := s.List()
	if err != nil {
		return nil, err
	}

	// Group by status
	byStatus := make(map[Status][]*Issue)
	for _, issue := range issues {
		byStatus[issue.Status] = append(byStatus[issue.Status], issue)
	}

	// Build columns in order
	board := &Board{}
	for _, status := range ValidStatuses() {
		board.Columns = append(board.Columns, Column{
			Status: status,
			Issues: byStatus[status],
		})
	}
	return board, nil
}

// GetChildren retrieves child issues of a parent.
func (s *Store) GetChildren(parentID string) ([]*Issue, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, status, priority, labels, assignee, parent_id, created_at, updated_at
		FROM issues WHERE parent_id = ? ORDER BY created_at
	`, parentID)
	if err != nil {
		return nil, fmt.Errorf("querying children: %w", err)
	}
	defer rows.Close()

	var issues []*Issue
	for rows.Next() {
		issue, err := scanIssueRows(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

// scanner interface for both sql.Row and sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanIssue(row *sql.Row) (*Issue, error) {
	var issue Issue
	var labels string
	var parentID sql.NullString

	err := row.Scan(
		&issue.ID, &issue.Title, &issue.Description,
		&issue.Status, &issue.Priority, &labels,
		&issue.Assignee, &parentID,
		&issue.CreatedAt, &issue.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning issue: %w", err)
	}

	if labels != "" {
		if err := json.Unmarshal([]byte(labels), &issue.Labels); err != nil {
			return nil, fmt.Errorf("unmarshaling labels: %w", err)
		}
	}
	if parentID.Valid {
		issue.ParentID = parentID.String
	}

	return &issue, nil
}

func scanIssueRows(rows *sql.Rows) (*Issue, error) {
	var issue Issue
	var labels string
	var parentID sql.NullString

	err := rows.Scan(
		&issue.ID, &issue.Title, &issue.Description,
		&issue.Status, &issue.Priority, &labels,
		&issue.Assignee, &parentID,
		&issue.CreatedAt, &issue.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning issue: %w", err)
	}

	if labels != "" {
		if err := json.Unmarshal([]byte(labels), &issue.Labels); err != nil {
			return nil, fmt.Errorf("unmarshaling labels: %w", err)
		}
	}
	if parentID.Valid {
		issue.ParentID = parentID.String
	}

	return &issue, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func generateID() string {
	// Short random ID: 8 hex chars (4 bytes = 32 bits)
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp
		return fmt.Sprintf("i%d", time.Now().UnixNano()%1000000)
	}
	return hex.EncodeToString(b)
}

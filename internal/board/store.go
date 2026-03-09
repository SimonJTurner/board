package board

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	boardDirName       = ".board"
	boardFileName      = "board.json"
	startingIssueValue = 1001
)

var multiUnderscore = regexp.MustCompile(`_+`)

type Store struct{}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) InitProject(project string) (string, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return "", errors.New("project is required")
	}
	projectPath, err := s.projectPath(project)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(projectPath); err == nil {
		return "", fmt.Errorf("project already exists: %s", project)
	}
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		return "", err
	}
	meta := BoardMeta{
		Project:         project,
		ProjectSlug:     projectSlug(project),
		Version:         1,
		NextIssueNumber: startingIssueValue,
		Issues:          []IssueMeta{},
	}
	if err := s.writeBoard(projectPath, meta); err != nil {
		return "", err
	}
	return projectPath, nil
}

func (s *Store) CreateIssue(project, title, description, assignee string) (IssueMeta, error) {
	projectPath, meta, err := s.loadBoardForWrite(project)
	if err != nil {
		return IssueMeta{}, err
	}
	if strings.TrimSpace(title) == "" {
		return IssueMeta{}, errors.New("title is required")
	}

	now := time.Now().UTC()
	number := meta.NextIssueNumber
	titleSlug := normalizeToken(title)
	id := fmt.Sprintf("%s_%d_%s", meta.ProjectSlug, number, titleSlug)
	filename := id + ".md"
	doc := IssueDoc{
		ID:          id,
		Number:      number,
		Title:       title,
		Status:      StatusTodo,
		Assignee:    strings.TrimSpace(assignee),
		CreatedAt:   now,
		UpdatedAt:   now,
		Description: description,
	}
	if err := s.writeIssue(projectPath, filename, doc); err != nil {
		return IssueMeta{}, err
	}

	issueMeta := IssueMeta{
		ID:                  id,
		Number:              number,
		File:                filename,
		Title:               title,
		Status:              StatusTodo,
		Assignee:            doc.Assignee,
		DescriptionChecksum: checksum(description),
		UpdatedAt:           now,
	}
	meta.Issues = append(meta.Issues, issueMeta)
	meta.NextIssueNumber++
	if err := s.writeBoard(projectPath, meta); err != nil {
		return IssueMeta{}, err
	}
	return issueMeta, nil
}

func (s *Store) AssignIssue(project, id, assignee string, status *string) (IssueMeta, string, error) {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" {
		return IssueMeta{}, "", errors.New("assignee is required")
	}
	projectPath, meta, err := s.loadBoardForWrite(project)
	if err != nil {
		return IssueMeta{}, "", err
	}
	idx := findIssueIndex(meta.Issues, id)
	if idx == -1 {
		return IssueMeta{}, "", issueNotFoundErr(project, id)
	}
	m := meta.Issues[idx]
	doc, err := s.readIssue(projectPath, m.File)
	if err != nil {
		return IssueMeta{}, "", err
	}
	oldAssignee := doc.Assignee
	doc.Assignee = assignee
	doc.UpdatedAt = time.Now().UTC()
	// Default to in_progress unless --status is provided
	newStatus := StatusInProgress
	if status != nil && strings.TrimSpace(*status) != "" {
		sTrim := strings.TrimSpace(*status)
		if !AllowedStatuses[sTrim] {
			return IssueMeta{}, "", fmt.Errorf("invalid status %q (allowed: todo, in_progress, done, cancelled)", sTrim)
		}
		newStatus = sTrim
	}
	doc.Status = newStatus
	m.Status = newStatus
	if err := s.writeIssue(projectPath, m.File, doc); err != nil {
		return IssueMeta{}, "", err
	}
	m.Assignee = assignee
	m.UpdatedAt = doc.UpdatedAt
	meta.Issues[idx] = m
	if err := s.writeBoard(projectPath, meta); err != nil {
		return IssueMeta{}, "", err
	}
	return m, oldAssignee, nil
}

func (s *Store) UpdateIssue(project, id string, updates IssueUpdateInput) (IssueMeta, IssueMeta, error) {
	projectPath, meta, err := s.loadBoardForWrite(project)
	if err != nil {
		return IssueMeta{}, IssueMeta{}, err
	}
	idx := findIssueIndex(meta.Issues, id)
	if idx == -1 {
		return IssueMeta{}, IssueMeta{}, issueNotFoundErr(project, id)
	}
	old := meta.Issues[idx]
	newMeta := old
	doc, err := s.readIssue(projectPath, old.File)
	if err != nil {
		return IssueMeta{}, IssueMeta{}, err
	}

	changed := false
	if updates.Title != nil {
		t := strings.TrimSpace(*updates.Title)
		if t == "" {
			return IssueMeta{}, IssueMeta{}, errors.New("title cannot be empty")
		}
		if t != doc.Title {
			doc.Title = t
			newMeta.Title = t
			changed = true
		}
	}
	if updates.Status != nil {
		s := strings.TrimSpace(*updates.Status)
		if !AllowedStatuses[s] {
			return IssueMeta{}, IssueMeta{}, fmt.Errorf("invalid status %q (allowed: todo, in_progress, done, cancelled)", s)
		}
		if s != doc.Status {
			doc.Status = s
			newMeta.Status = s
			changed = true
		}
	}
	if updates.Description != nil {
		d := *updates.Description
		if d != doc.Description {
			doc.Description = d
			newMeta.DescriptionChecksum = checksum(d)
			changed = true
		}
	}
	if updates.Assignee != nil {
		a := strings.TrimSpace(*updates.Assignee)
		if a != doc.Assignee {
			doc.Assignee = a
			newMeta.Assignee = a
			changed = true
		}
	}
	if !changed {
		return old, old, nil
	}
	doc.UpdatedAt = time.Now().UTC()
	newMeta.UpdatedAt = doc.UpdatedAt
	if err := s.writeIssue(projectPath, old.File, doc); err != nil {
		return IssueMeta{}, IssueMeta{}, err
	}
	meta.Issues[idx] = newMeta
	if err := s.writeBoard(projectPath, meta); err != nil {
		return IssueMeta{}, IssueMeta{}, err
	}
	return old, newMeta, nil
}

func (s *Store) ListIssues(project string) ([]IssueMeta, error) {
	_, meta, err := s.loadBoard(project)
	if err != nil {
		return nil, err
	}
	issues := make([]IssueMeta, len(meta.Issues))
	copy(issues, meta.Issues)
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Number < issues[j].Number
	})
	return issues, nil
}

func (s *Store) LoadBoard(project string) (BoardMeta, error) {
	_, meta, err := s.loadBoard(project)
	return meta, err
}

func (s *Store) ListProjects(includeArchived bool) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, boardDirName)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}

	projects := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project := entry.Name()
		if project == archiveDirName {
			if includeArchived {
				archives, _ := s.listArchiveProjects()
				projects = append(projects, archives...)
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(root, project, boardFileName)); err == nil {
			projects = append(projects, project)
		}
	}
	sort.Strings(projects)
	return projects, nil
}

const archiveDirName = "_archive"

func (s *Store) listArchiveProjects() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, boardDirName, archiveDirName)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	projects := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project := entry.Name()
		if _, err := os.Stat(filepath.Join(root, project, boardFileName)); err == nil {
			projects = append(projects, project)
		}
	}
	sort.Strings(projects)
	return projects, nil
}

func (s *Store) ArchiveProject(project string) error {
	projectPath, err := s.projectPath(project)
	if err != nil {
		return err
	}
	if _, err := os.Stat(projectPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("project not found: %s", project)
		}
		return err
	}
	archiveRoot, err := s.archivePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(archiveRoot, project)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("archive already contains %s", project)
	}
	return os.Rename(projectPath, dest)
}

func (s *Store) archivePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, boardDirName, archiveDirName), nil
}

func (s *Store) DeleteProject(project string) error {
	projectPath, err := s.projectPath(project)
	if err != nil {
		return err
	}
	if _, err := os.Stat(projectPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("project not found: %s", project)
		}
		return err
	}
	return os.RemoveAll(projectPath)
}

func (s *Store) GetIssue(project, id string) (IssueDoc, IssueMeta, error) {
	projectPath, meta, err := s.loadBoard(project)
	if err != nil {
		return IssueDoc{}, IssueMeta{}, err
	}
	idx := findIssueIndex(meta.Issues, id)
	if idx == -1 {
		return IssueDoc{}, IssueMeta{}, issueNotFoundErr(project, id)
	}
	m := meta.Issues[idx]
	doc, err := s.readIssue(projectPath, m.File)
	if err != nil {
		return IssueDoc{}, IssueMeta{}, err
	}
	return doc, m, nil
}

func (s *Store) GetIssueFilePath(project, id string) (string, error) {
	projectPath, meta, err := s.loadBoard(project)
	if err != nil {
		return "", err
	}
	idx := findIssueIndex(meta.Issues, id)
	if idx == -1 {
		return "", issueNotFoundErr(project, id)
	}
	return filepath.Join(projectPath, meta.Issues[idx].File), nil
}

func (s *Store) loadBoard(project string) (string, BoardMeta, error) {
	projectPath, err := s.projectPath(project)
	if err != nil {
		return "", BoardMeta{}, err
	}
	meta, err := s.readBoard(projectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", BoardMeta{}, projectNotFoundErr(project, err)
		}
		return "", BoardMeta{}, err
	}
	return projectPath, meta, nil
}

// projectNotFoundErr returns a user-friendly error when the project is missing,
// or the raw error when BOARD_DEBUG is set.
func projectNotFoundErr(project string, underlying error) error {
	if os.Getenv("BOARD_DEBUG") != "" {
		return underlying
	}
	return fmt.Errorf("project %q not found; run: board init %s", project, project)
}

// issueNotFoundErr returns a user-friendly message for missing issues.
func issueNotFoundErr(project, id string) error {
	return fmt.Errorf("issue %q not found in project %q; list issues with: board issue list %s", id, project, project)
}

func (s *Store) loadBoardForWrite(project string) (string, BoardMeta, error) {
	return s.loadBoard(project)
}

func (s *Store) projectPath(project string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	project = strings.TrimSpace(project)
	if project == "" {
		return "", errors.New("project is required")
	}
	return filepath.Join(home, boardDirName, project), nil
}

func (s *Store) boardPath(projectPath string) string {
	return filepath.Join(projectPath, boardFileName)
}

func (s *Store) readBoard(projectPath string) (BoardMeta, error) {
	p := s.boardPath(projectPath)
	b, err := os.ReadFile(p)
	if err != nil {
		return BoardMeta{}, err
	}
	var meta BoardMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return BoardMeta{}, err
	}
	return meta, nil
}

func (s *Store) writeBoard(projectPath string, meta BoardMeta) error {
	p := s.boardPath(projectPath)
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return atomicWrite(p, b, 0o644)
}

func (s *Store) readIssue(projectPath, file string) (IssueDoc, error) {
	path := filepath.Join(projectPath, file)
	b, err := os.ReadFile(path)
	if err != nil {
		return IssueDoc{}, err
	}
	return parseIssueMarkdown(string(b))
}

func (s *Store) writeIssue(projectPath, file string, doc IssueDoc) error {
	path := filepath.Join(projectPath, file)
	content := formatIssueMarkdown(doc)
	return atomicWrite(path, []byte(content), 0o644)
}

func atomicWrite(path string, b []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func findIssueIndex(issues []IssueMeta, id string) int {
	for i := range issues {
		if issues[i].ID == id {
			return i
		}
	}
	return -1
}

func normalizeToken(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	var b strings.Builder
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	out = multiUnderscore.ReplaceAllString(out, "_")
	if out == "" {
		return "untitled"
	}
	return out
}

func projectSlug(project string) string {
	t := normalizeToken(project)
	if t == "" {
		return "PROJECT"
	}
	return strings.ToUpper(t)
}

func checksum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func parseIssueMarkdown(content string) (IssueDoc, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return IssueDoc{}, errors.New("issue markdown missing front matter start")
	}
	parts := strings.SplitN(content, "\n---\n", 2)
	if len(parts) != 2 {
		return IssueDoc{}, errors.New("issue markdown missing front matter delimiter")
	}
	head := strings.TrimPrefix(parts[0], "---\n")
	body := parts[1]
	if strings.HasSuffix(body, "\n") {
		body = strings.TrimSuffix(body, "\n")
	}
	fields := map[string]string{}
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		fields[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}

	number, _ := strconv.Atoi(fields["number"])
	createdAt, _ := time.Parse(time.RFC3339, fields["created_at"])
	updatedAt, _ := time.Parse(time.RFC3339, fields["updated_at"])
	if fields["id"] == "" {
		return IssueDoc{}, errors.New("issue markdown missing id")
	}
	if fields["title"] == "" {
		return IssueDoc{}, errors.New("issue markdown missing title")
	}
	if fields["status"] == "" {
		fields["status"] = StatusTodo
	}
	return IssueDoc{
		ID:          fields["id"],
		Number:      number,
		Title:       fields["title"],
		Status:      fields["status"],
		Assignee:    fields["assignee"],
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		Description: body,
	}, nil
}

func formatIssueMarkdown(doc IssueDoc) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + doc.ID + "\n")
	b.WriteString("number: " + strconv.Itoa(doc.Number) + "\n")
	b.WriteString("title: " + doc.Title + "\n")
	b.WriteString("status: " + doc.Status + "\n")
	b.WriteString("assignee: " + doc.Assignee + "\n")
	b.WriteString("created_at: " + doc.CreatedAt.Format(time.RFC3339) + "\n")
	b.WriteString("updated_at: " + doc.UpdatedAt.Format(time.RFC3339) + "\n")
	b.WriteString("---\n")
	b.WriteString(doc.Description)
	if !strings.HasSuffix(doc.Description, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

type IssueUpdateInput struct {
	Title       *string
	Status      *string
	Description *string
	Assignee    *string
}

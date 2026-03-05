package board

import "time"

const (
	StatusTodo       = "todo"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusCancelled  = "cancelled"
)

var AllowedStatuses = map[string]bool{
	StatusTodo:       true,
	StatusInProgress: true,
	StatusDone:       true,
	StatusCancelled:  true,
}

const (
	EventIssueCreated            = "issue_created"
	EventIssueStatusChanged      = "issue_status_changed"
	EventIssueAssigneeChanged    = "issue_assignee_changed"
	EventIssueTitleChanged       = "issue_title_changed"
	EventIssueDescriptionChanged = "issue_description_changed"
)

// DefaultEnabledEventTypes centralizes event enablement so opting out later is
// a one-line change.
var DefaultEnabledEventTypes = map[string]bool{
	EventIssueCreated:            true,
	EventIssueStatusChanged:      true,
	EventIssueAssigneeChanged:    true,
	EventIssueTitleChanged:       true,
	EventIssueDescriptionChanged: true,
}

type BoardMeta struct {
	Project         string      `json:"project"`
	ProjectSlug     string      `json:"project_slug"`
	Version         int         `json:"version"`
	NextIssueNumber int         `json:"next_issue_number"`
	Issues          []IssueMeta `json:"issues"`
}

type IssueMeta struct {
	ID                  string    `json:"id"`
	Number              int       `json:"number"`
	File                string    `json:"file"`
	Title               string    `json:"title"`
	Status              string    `json:"status"`
	Assignee            string    `json:"assignee,omitempty"`
	DescriptionChecksum string    `json:"description_checksum"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type IssueDoc struct {
	ID          string
	Number      int
	Title       string
	Status      string
	Assignee    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Description string
}

type Event struct {
	Type        string    `json:"type"`
	Project     string    `json:"project"`
	IssueID     string    `json:"issue_id"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Status      string    `json:"status,omitempty"`
	OldStatus   string    `json:"old_status,omitempty"`
	NewStatus   string    `json:"new_status,omitempty"`
	Assignee    string    `json:"assignee,omitempty"`
	OldAssignee string    `json:"old_assignee,omitempty"`
	NewAssignee string    `json:"new_assignee,omitempty"`
	OldTitle    string    `json:"old_title,omitempty"`
	NewTitle    string    `json:"new_title,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

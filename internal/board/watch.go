package board

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"time"
)

type WatchConfig struct {
	Project   string
	Interval  time.Duration
	HookCmd   string
	EnableMap map[string]bool
}

func Watch(ctx context.Context, store *Store, cfg WatchConfig) error {
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	if cfg.EnableMap == nil {
		cfg.EnableMap = DefaultEnabledEventTypes
	}

	prev, err := store.LoadBoard(cfg.Project)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			curr, err := store.LoadBoard(cfg.Project)
			if err != nil {
				log.Printf("watch: failed loading board: %v", err)
				continue
			}
			events := diffIssues(prev, curr, cfg.EnableMap)
			for _, ev := range events {
				emitEvent(ev, cfg.HookCmd)
			}
			prev = curr
		}
	}
}

func diffIssues(prev, curr BoardMeta, enabled map[string]bool) []Event {
	prevByID := make(map[string]IssueMeta, len(prev.Issues))
	for _, m := range prev.Issues {
		prevByID[m.ID] = m
	}
	currByID := make(map[string]IssueMeta, len(curr.Issues))
	for _, m := range curr.Issues {
		currByID[m.ID] = m
	}

	now := time.Now().UTC()
	events := make([]Event, 0)
	for id, cur := range currByID {
		old, existed := prevByID[id]
		if !existed {
			if enabled[EventIssueCreated] {
				events = append(events, Event{
					Type:      EventIssueCreated,
					Project:   curr.Project,
					IssueID:   cur.ID,
					Number:    cur.Number,
					Title:     cur.Title,
					Status:    cur.Status,
					Assignee:  cur.Assignee,
					Timestamp: now,
				})
			}
			continue
		}

		if old.Status != cur.Status && enabled[EventIssueStatusChanged] {
			events = append(events, Event{
				Type:      EventIssueStatusChanged,
				Project:   curr.Project,
				IssueID:   cur.ID,
				Number:    cur.Number,
				Title:     cur.Title,
				OldStatus: old.Status,
				NewStatus: cur.Status,
				Timestamp: now,
			})
		}
		if old.Assignee != cur.Assignee && enabled[EventIssueAssigneeChanged] {
			events = append(events, Event{
				Type:        EventIssueAssigneeChanged,
				Project:     curr.Project,
				IssueID:     cur.ID,
				Number:      cur.Number,
				Title:       cur.Title,
				OldAssignee: old.Assignee,
				NewAssignee: cur.Assignee,
				Timestamp:   now,
			})
		}
		if old.Title != cur.Title && enabled[EventIssueTitleChanged] {
			events = append(events, Event{
				Type:      EventIssueTitleChanged,
				Project:   curr.Project,
				IssueID:   cur.ID,
				Number:    cur.Number,
				Title:     cur.Title,
				OldTitle:  old.Title,
				NewTitle:  cur.Title,
				Timestamp: now,
			})
		}
		if old.DescriptionChecksum != cur.DescriptionChecksum && enabled[EventIssueDescriptionChanged] {
			events = append(events, Event{
				Type:      EventIssueDescriptionChanged,
				Project:   curr.Project,
				IssueID:   cur.ID,
				Number:    cur.Number,
				Title:     cur.Title,
				Timestamp: now,
			})
		}
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].Number == events[j].Number {
			return events[i].Type < events[j].Type
		}
		return events[i].Number < events[j].Number
	})
	return events
}

func emitEvent(ev Event, hookCmd string) {
	b, err := json.Marshal(ev)
	if err != nil {
		log.Printf("watch: marshal event failed: %v", err)
		return
	}
	fmt.Println(string(b))
	if hookCmd == "" {
		return
	}
	cmd := exec.Command("sh", "-c", hookCmd)
	cmd.Stdin = bytes.NewReader(b)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("watch: hook failed (%s): %v; output=%s", hookCmd, err, string(out))
	}
}

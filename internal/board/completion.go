package board

import (
	"fmt"
	"sort"
	"strings"
)

const (
	compShellBash = "bash"
	compShellZsh  = "zsh"
)

func runCompletion(store *Store, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: board completion <bash|zsh>")
	}
	shell := strings.TrimSpace(strings.ToLower(args[0]))
	switch shell {
	case compShellBash:
		fmt.Print(completionScriptBash())
		return nil
	case compShellZsh:
		fmt.Print(completionScriptZsh())
		return nil
	default:
		return fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
	}
}

func runComplete(store *Store, args []string) error {
	// args: shell, comp_line, comp_cword (shell unused, both bash and zsh use same protocol)
	if len(args) < 3 {
		return nil
	}
	compLine := args[1]
	compCwordStr := args[2]
	var compCword int
	if _, err := fmt.Sscanf(compCwordStr, "%d", &compCword); err != nil {
		return nil
	}
	words := splitCompLine(compLine)
	if compCword < 0 || compCword > len(words) {
		return nil
	}
	prefix := ""
	if compCword < len(words) {
		prefix = words[compCword]
	}

	candidates := completeWords(store, words, compCword, prefix)
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			fmt.Println(c)
		}
	}
	return nil
}

func splitCompLine(line string) []string {
	var out []string
	for _, s := range strings.Fields(line) {
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func completeWords(store *Store, words []string, cword int, prefix string) []string {
	// words[0] is "board", words[1] is subcommand, etc.
	if cword <= 0 {
		return nil
	}
	if cword == 1 {
		return filterPrefix([]string{"init", "project", "update", "issue", "watch", "help", "-h", "--help"}, prefix)
	}
	cmd := words[1]
	switch cmd {
	case "init":
		if cword == 2 {
			return filterPrefix(projectList(store), prefix)
		}
		return nil
	case "project":
		if cword == 2 {
			return filterPrefix([]string{"list", "delete", "archive"}, prefix)
		}
		if cword == 3 && (words[2] == "delete" || words[2] == "archive") {
			return filterPrefix(projectList(store), prefix)
		}
		if cword == 3 && words[2] == "list" {
			return filterPrefix([]string{"--archived"}, prefix)
		}
		return nil
	case "update":
		if cword == 2 {
			return filterPrefix([]string{"--repo", "--release-repo"}, prefix)
		}
		return nil
	case "issue":
		if cword == 2 {
			return filterPrefix([]string{"create", "assign", "update", "list", "next", "show"}, prefix)
		}
		sub := words[2]
		if sub == "create" {
			if cword == 3 {
				out := append([]string(nil), projectList(store)...)
				out = append(out, "--title", "--description", "--assignee", "--json")
				return filterPrefix(out, prefix)
			}
			return nil
		}
		if sub == "list" || sub == "next" {
			if cword == 3 {
				out := append([]string(nil), projectList(store)...)
				out = append(out, "--status", "--limit", "--json")
				return filterPrefix(out, prefix)
			}
			if cword >= 3 {
				for i := 3; i < len(words); i++ {
					if words[i] == "--status" && i+1 < len(words) && (i+1 == cword || strings.HasPrefix(words[i+1], "-")) {
						if i+1 == cword {
							return filterPrefix([]string{StatusTodo, StatusInProgress, StatusDone, StatusCancelled}, prefix)
						}
						break
					}
				}
			}
			return nil
		}
		if sub == "assign" || sub == "update" || sub == "show" {
			// [project] <issue-id> or <issue-id> then flags
			if cword == 3 {
				// could be project or issue-id
				out := append([]string(nil), projectList(store)...)
				issues := issueIDsForProjects(store, nil)
				out = append(out, issues...)
				return filterPrefix(out, prefix)
			}
			if cword == 4 {
				// if words[3] was project, this is issue-id; else we're in flags
				if len(projectList(store)) > 0 && sliceContains(projectList(store), words[3]) {
					return filterPrefix(issueIDsForProject(store, words[3]), prefix)
				}
				flags := []string{"--assignee", "--status", "--json"}
				if sub == "update" {
					flags = []string{"--title", "--description", "--assignee", "--status", "--json"}
				}
				if sub == "show" {
					flags = []string{"--json"}
				}
				return filterPrefix(flags, prefix)
			}
			if cword == 5 && (sub == "assign" || sub == "update") {
				for i := 3; i < len(words); i++ {
					if words[i] == "--status" && i+1 == cword {
						return filterPrefix([]string{StatusTodo, StatusInProgress, StatusDone, StatusCancelled}, prefix)
					}
				}
			}
			return nil
		}
		return nil
	case "watch":
		if cword == 2 {
			out := append([]string(nil), projectList(store)...)
			out = append(out, "--interval", "--hook-cmd", "--plain")
			return filterPrefix(out, prefix)
		}
		return nil
	case "help", "-h", "--help":
		return nil
	}
	return nil
}

func projectList(store *Store) []string {
	list, err := store.ListProjects(true)
	if err != nil {
		return nil
	}
	return list
}

func issueIDsForProject(store *Store, project string) []string {
	issues, err := store.ListIssues(project)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(issues))
	for _, m := range issues {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	return ids
}

func issueIDsForProjects(store *Store, projects []string) []string {
	if projects == nil {
		projects = projectList(store)
	}
	seen := make(map[string]bool)
	for _, p := range projects {
		for _, id := range issueIDsForProject(store, p) {
			seen[id] = true
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func sliceContains(slice []string, x string) bool {
	for _, s := range slice {
		if s == x {
			return true
		}
	}
	return false
}

func filterPrefix(candidates []string, prefix string) []string {
	if prefix == "" {
		return candidates
	}
	var out []string
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func completionScriptBash() string {
	return `# board bash completion
_board() {
  local cur
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  local comp_line="${COMP_LINE}"
  local comp_cword="${COMP_CWORD}"
  local IFS=$'\n'
  COMPREPLY=($(board __complete bash "$comp_line" "$comp_cword" 2>/dev/null))
  return 0
}
complete -o default -F _board board
`
}

func completionScriptZsh() string {
	return `# board zsh completion
_board() {
  local line cword reply
  line="${words[*]}"
  cword=$((CURRENT - 1))
  reply=($(board __complete bash "$line" "$cword" 2>/dev/null))
  [[ $#reply -gt 0 ]] && compadd -a reply
}
compdef _board board
`
}

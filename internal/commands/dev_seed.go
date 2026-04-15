package commands

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

const defaultSeedPath = "/tmp/ruin-test-vault"

func newSeedCmd(jsonOutput *bool) *cobra.Command {
	var clean, reset bool

	cmd := &cobra.Command{
		Use:   "seed [path]",
		Short: "Create a test vault with sample notes",
		Long: `Create a test vault with 113 sample notes for development and testing.

Generates 5 hub notes, 100 regular notes (daily logs, code notes,
meeting notes, ideas, and task lists), 5 link notes, and 3 embed notes
with realistic content, tags, parent relationships, wiki links, static
embeds, and dynamic embeds.`,
		Example: `  ruin dev seed                          # Create at /tmp/ruin-test-vault
  ruin dev seed ~/my-test-vault          # Create at custom path
  ruin dev seed --clean                  # Remove test vault
  ruin dev seed --reset                  # Clean and recreate`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := defaultSeedPath
			if len(args) > 0 {
				path = args[0]
			}

			// Expand ~ and make absolute
			if len(path) > 0 && path[0] == '~' {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to expand home directory: %w", err)
				}
				path = filepath.Join(home, path[1:])
			}
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}
			path = absPath

			if clean || reset {
				if err := cleanVault(path); err != nil {
					return err
				}
				if clean {
					return nil
				}
			}

			return createSeedVault(path)
		},
	}

	cmd.Flags().BoolVar(&clean, "clean", false, "remove the test vault")
	cmd.Flags().BoolVar(&reset, "reset", false, "clean and recreate the test vault")

	return cmd
}

func cleanVault(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Directory does not exist: %s\n", path)
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}

	// Safety check: only delete if .ruin/ directory exists
	ruinDir := filepath.Join(path, ".ruin")
	if _, err := os.Stat(ruinDir); os.IsNotExist(err) {
		return fmt.Errorf("not a ruin vault (no .ruin/ directory): %s\nRefusing to delete for safety", path)
	}

	fmt.Fprintf(os.Stderr, "Removing test vault: %s\n", path)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to remove vault: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Done.")
	return nil
}

func createSeedVault(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("directory already exists: %s\nUse --reset to recreate or --clean first", path)
	}

	fmt.Fprintf(os.Stderr, "Creating test vault at: %s\n", path)

	if err := os.MkdirAll(filepath.Join(path, ".ruin"), 0755); err != nil {
		return fmt.Errorf("failed to create vault directory: %w", err)
	}

	rng := rand.New(rand.NewSource(42)) // deterministic seed for reproducibility
	baseTime := time.Now().AddDate(0, 0, -30)

	// --- Create hub notes ---
	fmt.Fprintln(os.Stderr, "  Creating 5 hub notes...")
	type hubDef struct {
		project, title, desc string
	}
	hubs := []hubDef{
		{"alpha", "Project Alpha Hub", "Central tracking note for Project Alpha. All alpha-related work links back here."},
		{"beta", "Project Beta Hub", "Project Beta coordination point. Design decisions and progress tracked below."},
		{"infra", "Infrastructure Hub", "Infrastructure initiatives and operational improvements."},
		{"docs", "Documentation Hub", "Documentation strategy, style guides, and content tracking."},
		{"platform", "Platform Hub", "Platform team hub -- shared services, libraries, and tooling."},
	}

	for i, h := range hubs {
		order := i + 1
		n := &note.Note{
			UUID:     fmt.Sprintf("test-uuid-hub-%s", h.project),
			Created:  baseTime.Add(time.Duration(i) * time.Hour),
			Updated:  baseTime.Add(time.Duration(i) * time.Hour),
			Order:    &order,
			FilePath: filepath.Join(path, note.SanitizeFilename(h.title)+".md"),
			Content: fmt.Sprintf(`# %s

#project #%s

%s

## Status

Actively maintained.

## Key Decisions

- Tracked in sub-notes linked via parent.
`, h.title, h.project, h.desc),
		}
		n.RefreshTags()
		if err := n.Save(); err != nil {
			return fmt.Errorf("failed to save hub note %s: %w", h.title, err)
		}
		fmt.Fprintf(os.Stderr, "    %s.md (%s)\n", note.SanitizeFilename(h.title), n.UUID)
	}

	// --- Content pools ---
	categories := []string{"daily", "work", "personal", "idea", "project", "meeting", "code", "review", "bug", "design", "infra", "docs", "platform"}
	projects := []string{"alpha", "beta", "infra", "docs", "platform"}
	statuses := []string{"draft", "wip", "done", "blocked", "urgent"}

	dailyOpenings := []string{
		"Started the morning with coffee and planning.",
		"Quiet start to the day, reviewing yesterday's progress.",
		"Monday energy -- jumped straight into deep work.",
		"Back from vacation, catching up on everything.",
		"Rainy day, perfect for focused coding.",
	}
	dailyTasks := []string{
		"Review open PRs", "Update project board", "Write weekly summary",
		"Sync with team lead", "Refactor auth module", "Fix CI pipeline",
		"Deploy staging build", "Write unit tests", "Update docs", "Triage new issues",
	}
	dailyClosings := []string{
		"Wrapped up early today, good progress overall.",
		"Long day but productive. Need to follow up on the blocked items tomorrow.",
		"Decent day. Left some loose ends for tomorrow.",
		"Solid day of shipping. Feeling good about the sprint.",
		"Ended with a few rabbit holes. Need to time-box better.",
	}

	codeDescriptions := []string{
		"Refactored the error handling in the API layer.",
		"Added retry logic with exponential backoff.",
		"Migrated the database schema for multi-tenancy.",
		"Implemented streaming response for large payloads.",
		"Fixed race condition in the worker pool.",
		"Added circuit breaker to external service calls.",
		"Optimized query performance with proper indexing.",
		"Extracted shared logic into a reusable package.",
		"Converted callbacks to async/await pattern.",
		"Added structured logging with correlation IDs.",
	}
	codeSnippets := []string{
		"The key insight was using a channel-based approach instead of mutexes.",
		"Using a decorator pattern here keeps the core logic clean.",
		"The trick is to batch the writes and flush on a timer.",
		"Interface segregation made testing much simpler.",
		"Moved validation to the boundary -- fail fast, fail early.",
	}
	codeTodos := []string{
		"Add integration tests for the happy path",
		"Benchmark the new implementation",
		"Get code review from the platform team",
		"Check if this breaks backward compat",
		"Add metrics for the new endpoint",
	}

	meetingTopics := []string{
		"Sprint planning for Q2", "Architecture review for new service",
		"Incident postmortem -- outage on Feb 1", "Cross-team sync on shared infrastructure",
		"Onboarding plan for new hires", "Dependency upgrade strategy",
		"Performance budget discussion", "Feature flag rollout plan",
		"API versioning approach", "Security audit findings",
	}
	meetingAttendees := []string{
		"Alice, Bob, Charlie", "Dana, Eve, Frank", "Grace, Hank",
		"Ivy, Jake, Kim, Leo", "Mona, Nick, Olivia",
	}
	meetingActions := []string{
		"Follow up on the timeline estimate",
		"Share the design doc with stakeholders",
		"Schedule a deeper dive on caching",
		"Create tickets for the agreed work",
		"Update the runbook with new procedures",
	}

	ideaSparks := []string{
		"What if we exposed the CLI as a local HTTP API?",
		"Could we auto-suggest tags based on content similarity?",
		"Graph visualization of note relationships.",
		"A weekly digest command that summarizes recent notes.",
		"Plugin system for custom output formatters.",
		"Fuzzy search with typo tolerance.",
		"Note templates for common patterns.",
		"Time-based note clustering.",
		"Export to static site.",
		"Integration with external knowledge bases.",
	}
	ideaDetails := []string{
		"This would let other tools interact with the vault programmatically.",
		"The embedding approach could work if we keep the index small.",
		"Mermaid diagrams in the terminal might be too noisy -- maybe a web view.",
		"Would need to think about privacy -- all local, no cloud.",
		"Could use a simple scoring heuristic to start.",
	}

	taskItems := []string{
		"Set up monitoring for the new endpoint",
		"Write migration script for legacy data",
		"Review and merge the pending PRs",
		"Update the deployment checklist",
		"Add rate limiting to the public API",
		"Create load test scenarios",
		"Document the new config options",
		"Fix the flaky test in CI",
		"Audit third-party dependencies",
		"Prepare demo for stakeholder review",
	}

	// URL pools
	codeMdURLs := []string{
		"[PR #142: Fix retry logic](https://github.com/acme/api-server/pull/142)",
		"[PR #287: Add circuit breaker](https://github.com/acme/api-server/pull/287)",
		"[Issue #53: Streaming support](https://github.com/acme/platform/issues/53)",
		"[Go net/http docs](https://pkg.go.dev/net/http)",
		"[PR #91: Terraform modules](https://github.com/acme/infra/pull/91)",
	}
	codePlainURLs := []string{
		"https://github.com/acme/api-server/pull/142",
		"https://github.com/acme/api-server/pull/287",
		"https://github.com/acme/platform/issues/53",
		"https://github.com/acme/infra/pull/91",
		"https://pkg.go.dev/net/http",
	}
	meetingURLs := []string{
		"[Design Doc: Auth Overhaul](https://docs.google.com/document/d/1abc-fake-id)",
		"[Sprint Board](https://linear.app/acme/project/sprint-q2)",
		"[Incident Timeline](https://status.acme.dev/incidents/20260201)",
		"[Architecture RFC](https://docs.google.com/document/d/2def-fake-id)",
		"[Onboarding Checklist](https://notion.so/acme/onboarding-2026)",
	}
	ideaMdURLs := []string{
		"[Zettelkasten Method](https://martinfowler.com/articles/zettelkasten.html)",
		"[CLI Tool Design](https://jvns.ca/blog/2024/cli-tools/)",
		"[Knowledge Graphs at Scale](https://research.google/pubs/pub43438/)",
		"[Local-first Software](https://www.inkandswitch.com/local-first/)",
		"[Building a Second Brain](https://fortelabs.com/blog/basb-overview/)",
	}
	ideaPlainURLs := []string{
		"https://martinfowler.com/articles/zettelkasten.html",
		"https://jvns.ca/blog/2024/cli-tools/",
		"https://research.google/pubs/pub43438/",
		"https://arxiv.org/abs/2301.00001",
		"https://simonwillison.net/2025/notes-graph/",
	}
	dailyMdURLs := []string{
		"[Yesterday's standup notes](https://docs.google.com/document/d/3ghi-fake-id)",
		"[Sprint retro feedback](https://linear.app/acme/project/retro)",
		"[Team wiki](https://notion.so/acme/engineering-wiki)",
		"[CI Dashboard](https://ci.acme.dev/pipelines)",
		"[Monitoring Grafana](https://grafana.acme.dev/d/overview)",
	}

	pick := func(pool []string) string {
		return pool[rng.Intn(len(pool))]
	}

	hubTitleFor := func(proj string) string {
		switch proj {
		case "alpha":
			return "Project Alpha Hub"
		case "beta":
			return "Project Beta Hub"
		case "infra":
			return "Infrastructure Hub"
		case "docs":
			return "Documentation Hub"
		case "platform":
			return "Platform Hub"
		}
		return ""
	}

	// --- Generate 100 regular notes ---
	fmt.Fprintln(os.Stderr, "  Creating 100 notes...")

	for num := 1; num <= 100; num++ {
		uuid := fmt.Sprintf("test-uuid-%03d", num)
		daysAgo := rng.Intn(30)
		hour := rng.Intn(14) + 7
		minute := rng.Intn(60)
		created := time.Now().AddDate(0, 0, -daysAgo)
		created = time.Date(created.Year(), created.Month(), created.Day(), hour, minute, 0, 0, created.Location())
		datePart := created.Format("2006-01-02")

		cat1 := pick(categories)
		cat2 := pick(categories)
		status := pick(statuses)

		var (
			parent   string
			order    *int
			content  string
			filename string
		)

		// Determine note type: 25 daily, 25 code, 20 meeting, 15 idea, 15 task
		switch {
		case num <= 25: // daily
			title := fmt.Sprintf("Daily Log %s", datePart)
			filename = fmt.Sprintf("Daily-Log-%s-%d.md", datePart, num)
			opening := pick(dailyOpenings)
			task1 := pick(dailyTasks)
			task2 := pick(dailyTasks)
			task3 := pick(dailyTasks)
			closing := pick(dailyClosings)
			dailyLink := pick(dailyMdURLs)

			dailyInline := ""
			if rng.Intn(2) == 0 {
				dailyInline = "  #followup"
			}

			extraStatus := ""
			if rng.Intn(3) == 0 {
				extraStatus = fmt.Sprintf(" #%s", status)
			}

			content = fmt.Sprintf(`# %s

#daily #%s%s

%s

## Tasks
- %s%s
- %s
- %s

## Notes

Focused on #%s work today. Need to keep momentum.

Reference: %s

%s
`, title, cat1, extraStatus, opening, task1, dailyInline, task2, task3, cat2, dailyLink, closing)

		case num <= 50: // code
			proj := pick(projects)
			desc := pick(codeDescriptions)
			title := fmt.Sprintf("Code - %.40s", desc)
			filename = fmt.Sprintf("Code-Note-%03d.md", num)
			snippet := pick(codeSnippets)
			todo1 := pick(codeTodos)
			todo2 := pick(codeTodos)
			codeMdURL := pick(codeMdURLs)
			codePlainURL := pick(codePlainURLs)
			hubTitle := hubTitleFor(proj)

			codeInline := ""
			if rng.Intn(5) < 3 {
				codeInline = "  #todo"
			}

			statusTag := ""
			if rng.Intn(2) == 0 {
				statusTag = fmt.Sprintf("\n\n#%s", status)
			}

			// ~40% get a hub parent
			if rng.Intn(5) < 2 {
				parent = fmt.Sprintf("test-uuid-hub-%s", proj)
			}

			content = fmt.Sprintf(`# %s

#code #%s

%s

See [[%s]] for project context.

## Details

%s

Related: %s

## TODO
- %s%s
- %s
- Review %s%s
`, title, proj, desc, hubTitle, snippet, codeMdURL, todo1, codeInline, todo2, codePlainURL, statusTag)

		case num <= 70: // meeting
			topic := pick(meetingTopics)
			attendees := pick(meetingAttendees)
			action1 := pick(meetingActions)
			action2 := pick(meetingActions)
			title := fmt.Sprintf("Meeting - %s", topic)
			filename = fmt.Sprintf("Meeting-%03d.md", num)
			meetingLink := pick(meetingURLs)

			meetingInline := ""
			if rng.Intn(2) == 0 {
				meetingInline = "  #followup"
			}

			tagLine := fmt.Sprintf("#meeting #%s", cat1)
			if rng.Intn(3) == 0 {
				tagLine = fmt.Sprintf("#meeting notes#, #%s", cat1)
			}

			content = fmt.Sprintf(`# %s

%s

## Attendees
%s

## Discussion

%s

Discussed timelines and priorities. #work

Resources: %s

## Action Items
- %s%s
- %s
`, title, tagLine, attendees, topic, meetingLink, action1, meetingInline, action2)

		case num <= 85: // idea
			spark := pick(ideaSparks)
			detail := pick(ideaDetails)
			title := fmt.Sprintf("Idea - %s", spark)
			filename = fmt.Sprintf("Idea-%03d.md", num)
			ideaMdURL := pick(ideaMdURLs)
			ideaPlainURL := pick(ideaPlainURLs)

			draftTag := ""
			if rng.Intn(2) == 0 {
				draftTag = " #draft"
			}

			// Idea nesting: 72->71, 74->72, 76->74
			switch num {
			case 72:
				parent = "test-uuid-071"
			case 74:
				parent = "test-uuid-072"
			case 76:
				parent = "test-uuid-074"
			}

			ideaLink := ""
			if rng.Intn(3) == 0 {
				ideaLink = "\n\nRelated to [[Project Alpha Hub]]."
			}

			content = fmt.Sprintf(`# %s

#idea #%s%s

%s

Inspired by %s

## Thinking

%s%s

See also: %s

## Next Steps

Think about this more. Maybe prototype something. #draft
`, title, cat1, draftTag, spark, ideaMdURL, detail, ideaLink, ideaPlainURL)

		default: // task (86-100)
			proj := pick(projects)
			t1 := pick(taskItems)
			t2 := pick(taskItems)
			t3 := pick(taskItems)
			t4 := pick(taskItems)
			title := fmt.Sprintf("Tasks - %s (%s)", proj, datePart)
			filename = fmt.Sprintf("Tasks-%03d.md", num)

			taskInline := ""
			if rng.Intn(5) < 2 {
				taskInline = "  #question"
			}

			// ~40% get a hub parent
			if rng.Intn(5) < 2 {
				parent = fmt.Sprintf("test-uuid-hub-%s", proj)
			}

			// Task notes get sequential order values (1-15)
			o := num - 85
			order = &o

			content = fmt.Sprintf(`# %s

#work #%s #%s

## Open
- [ ] %s%s
- [ ] %s

## In Progress
- [x] %s

## Done
- [x] %s
`, title, proj, status, t1, taskInline, t2, t3, t4)
		}

		// Orphan parent tests (notes 98, 99, 100)
		switch num {
		case 98:
			parent = "test-uuid-orphan-parent-1"
		case 99:
			parent = "test-uuid-orphan-parent-2"
		case 100:
			parent = "test-uuid-orphan-parent-3"
		}

		n := &note.Note{
			UUID:     uuid,
			Created:  created,
			Updated:  created,
			Parent:   parent,
			Order:    order,
			FilePath: filepath.Join(path, filename),
			Content:  content,
		}
		n.RefreshTags()

		if err := n.Save(); err != nil {
			return fmt.Errorf("failed to save note %d: %w", num, err)
		}

		if num%25 == 0 {
			fmt.Fprintf(os.Stderr, "    Created %d / 100 notes...\n", num)
		}

		// Reset per-iteration state
		parent = ""
		order = nil
	}

	// --- Create link notes ---
	fmt.Fprintln(os.Stderr, "  Creating 5 link notes...")
	linkNotes := []struct {
		uuid, filename, content string
		parent                  string
	}{
		{
			uuid:     "test-uuid-link-001",
			filename: "Link-Go-Blog.md",
			content: `# Go 1.22 Release Notes

https://go.dev/blog/go1.22

#link #code

Great overview of the new features in Go 1.22.
`,
		},
		{
			uuid:     "test-uuid-link-002",
			filename: "Link-Minimal.md",
			content: `https://example.com/article

#link
`,
		},
		{
			uuid:     "test-uuid-link-003",
			filename: "Link-Annotated.md",
			content: `# Local-First Software

https://www.inkandswitch.com/local-first/

#link #idea #design

This paper changed how I think about data ownership. Key takeaway:
the best apps keep data on the user's device and sync peer-to-peer.
`,
		},
		{
			uuid:     "test-uuid-link-004",
			filename: "Link-With-Comment.md",
			content: `# CLI Tool Design Patterns

https://jvns.ca/blog/2024/cli-tools/

#link #code #docs

Julia's take on CLI UX is spot-on. We should adopt the "progressive disclosure" pattern for ruin's help text.
`,
		},
		{
			uuid:     "test-uuid-link-005",
			filename: "Link-With-Parent.md",
			content: `# Platform API Reference

https://docs.acme.dev/api/v2

#link #platform

Official API docs for the v2 platform endpoints.
`,
			parent: "test-uuid-hub-platform",
		},
	}

	for i, ln := range linkNotes {
		created := baseTime.Add(time.Duration(110+i) * time.Hour)
		n := &note.Note{
			UUID:     ln.uuid,
			Created:  created,
			Updated:  created,
			Parent:   ln.parent,
			FilePath: filepath.Join(path, ln.filename),
			Content:  ln.content,
		}
		n.RefreshTags()
		n.URL = n.ExtractURL()
		if err := n.Save(); err != nil {
			return fmt.Errorf("failed to save link note %s: %w", ln.filename, err)
		}
		fmt.Fprintf(os.Stderr, "    %s (%s)\n", ln.filename, n.UUID)
	}

	// --- Create notes with embeds ---
	fmt.Fprintln(os.Stderr, "  Creating 3 embed notes...")
	embedNotes := []struct {
		uuid, filename, content string
		parent                  string
	}{
		{
			uuid:     "test-uuid-embed-001",
			filename: "Dashboard.md",
			content: `# Dashboard

#project

## Open Follow-ups

![[pick: #followup !#done | group=parent]]

## Recent Daily Logs

![[search: #daily | format=list, limit=5, sort=created:desc]]

## Project Alpha Overview

![[compose: Project Alpha Hub]]
`,
		},
		{
			uuid:     "test-uuid-embed-002",
			filename: "Weekly Review.md",
			content: `# Weekly Review

#review

## This Week's Meetings

![[search: #meeting | format=summary, limit=5]]

## Open Tasks

![[pick: #todo !#done]]

## Saved Query: Open Follow-ups

![[query: open-followups]]
`,
		},
		{
			uuid:     "test-uuid-embed-003",
			filename: "Alpha Compose.md",
			parent:   "test-uuid-hub-alpha",
			content: `# Alpha Compose

#alpha

Overview of Project Alpha work.

![[Project Alpha Hub]]

## Related Code

![[search: #code #alpha | format=list]]
`,
		},
	}

	for _, en := range embedNotes {
		created := baseTime.Add(120 * time.Hour)
		n := &note.Note{
			UUID:     en.uuid,
			Created:  created,
			Updated:  created,
			Parent:   en.parent,
			FilePath: filepath.Join(path, en.filename),
			Content:  en.content,
		}
		n.RefreshTags()
		if err := n.Save(); err != nil {
			return fmt.Errorf("failed to save embed note %s: %w", en.filename, err)
		}
		fmt.Fprintf(os.Stderr, "    %s (%s)\n", en.filename, n.UUID)
	}

	// --- Write queries.yml ---
	fmt.Fprintln(os.Stderr, "  Writing saved queries...")
	queriesYML := `queries:
    - name: daily-work
      query: '#daily #work'
    - name: active-bugs
      query: '#bug !#done'
    - name: project-alpha
      query: '#alpha'
    - name: recent-meetings
      query: '#meeting created:this-month'
    - name: open-followups
      query: '#followup !#done'
`
	if err := os.WriteFile(filepath.Join(path, ".ruin", "queries.yml"), []byte(queriesYML), 0644); err != nil {
		return fmt.Errorf("failed to write queries.yml: %w", err)
	}

	// --- Write parents.yml ---
	fmt.Fprintln(os.Stderr, "  Writing saved parent bookmarks...")
	parentsYML := `parents:
    - name: alpha
      uuid: test-uuid-hub-alpha
    - name: beta
      uuid: test-uuid-hub-beta
    - name: infra
      uuid: test-uuid-hub-infra
    - name: docs
      uuid: test-uuid-hub-docs
    - name: platform
      uuid: test-uuid-hub-platform
`
	if err := os.WriteFile(filepath.Join(path, ".ruin", "parents.yml"), []byte(parentsYML), 0644); err != nil {
		return fmt.Errorf("failed to write parents.yml: %w", err)
	}

	// --- Run doctor to build indexes ---
	fmt.Fprintln(os.Stderr, "  Building indexes...")
	vlt := vault.New(path)
	if _, err := vlt.Initialize(false); err != nil {
		// Already created .ruin, so Initialize may report files exist -- that's fine
		_ = err
	}

	// Run doctorFullScan directly to build tags.yml and titles.json
	if err := doctorFullScan(vlt, false, false); err != nil {
		return fmt.Errorf("failed to build indexes: %w", err)
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "Done. Created 113 notes at: %s\n", path)
	fmt.Fprintln(os.Stderr, "  - 25 daily logs")
	fmt.Fprintln(os.Stderr, "  - 25 code notes")
	fmt.Fprintln(os.Stderr, "  - 20 meeting notes")
	fmt.Fprintln(os.Stderr, "  - 15 idea notes")
	fmt.Fprintln(os.Stderr, "  - 15 task lists")
	fmt.Fprintln(os.Stderr, "  - 5 hub (project) notes")
	fmt.Fprintln(os.Stderr, "  - 5 link notes")
	fmt.Fprintln(os.Stderr, "  - 3 embed notes (Dashboard, Weekly Review, Alpha Compose)")
	fmt.Fprintln(os.Stderr, "  - 3 notes with orphaned parent references")
	fmt.Fprintln(os.Stderr, "  - Wiki links, static embeds, and dynamic embeds")
	fmt.Fprintln(os.Stderr, "  - 5 saved queries in queries.yml")
	fmt.Fprintln(os.Stderr, "  - 5 saved parent bookmarks (alpha, beta, infra, docs, platform)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "Test with:\n")
	fmt.Fprintf(os.Stderr, "  ruin --vault %s log\n", path)
	fmt.Fprintf(os.Stderr, "  ruin --vault %s search \"#daily\"\n", path)
	fmt.Fprintf(os.Stderr, "  ruin --vault %s compose Dashboard --expand-embeds\n", path)
	fmt.Fprintf(os.Stderr, "  ruin --vault %s pick \"#followup\" \"!#done\"\n", path)
	fmt.Fprintf(os.Stderr, "  ruin --vault %s query run open-followups\n", path)

	return nil
}

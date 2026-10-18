package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

// history is what changed over a period: the commands answered (or answered
// better) than at the base commit, the last one before the period.
type history struct {
	Since   string         `json:"since"`
	Base    string         `json:"base,omitempty"`
	Gained  int            `json:"gained"` // commands answered that were missing
	Gains   map[uint32]int `json:"gains"`  // by module
	Commits []*commit      `json:"commits"`
	More    int            `json:"more,omitempty"` // commits left out of the list
}

// commit is what one commit changed in one module.
type commit struct {
	Rev     string    `json:"rev"`
	Time    time.Time `json:"time"`
	Summary string    `json:"summary"`
	Module  uint32    `json:"module"`
	Label   string    `json:"label,omitempty"`
	Changes []change  `json:"changes"`
}

type change struct {
	ID  uint32 `json:"id"`
	Was status `json:"was"`
	Now status `json:"now"`
}

// rank orders the statuses from no handler to saved state.
var rank = map[status]int{statusMissing: 0, statusStub: 1, statusStatic: 2, statusStateful: 3}

// changes compares rep with the base commit of the last period and lists the
// commits behind each change, newest first, at most limit of them.
func changes(rep *report, period time.Duration, limit int) *history {
	h := &history{Since: periodLabel(period), Gains: map[uint32]int{}}
	h.Base = git("rev-list", "-1", "--before=@"+strconv.FormatInt(time.Now().Add(-period).Unix(), 10), "HEAD")
	before := baseline(h.Base)

	labels := map[uint32]string{}
	for _, m := range rep.Modules {
		labels[m.ID] = m.Label
	}
	byKey := map[string]*commit{}
	for _, p := range rep.Packets {
		was := statusMissing
		if old := before[p.ID]; old != nil {
			was = old.Status
		}
		if rank[p.Status] <= rank[was] {
			continue
		}
		if was == statusMissing {
			h.Gained++
			h.Gains[p.ID/1000]++
		}
		c := introducedBy(p, h.Base)
		key := c.Rev + "/" + strconv.Itoa(int(p.ID/1000))
		if byKey[key] == nil {
			c.Module, c.Label = p.ID/1000, labels[p.ID/1000]
			byKey[key] = c
			h.Commits = append(h.Commits, c)
		}
		byKey[key].Changes = append(byKey[key].Changes, change{ID: p.ID, Was: was, Now: p.Status})
	}
	slices.SortStableFunc(h.Commits, func(a, b *commit) int {
		if !a.Time.Equal(b.Time) {
			return b.Time.Compare(a.Time)
		}
		return int(a.Module) - int(b.Module)
	})
	if len(h.Commits) > limit {
		h.More = len(h.Commits) - limit
		h.Commits = h.Commits[:limit]
	}
	return h
}

// baseline classifies the handlers of rev, checked out in a temporary
// worktree. Nothing when there is no such commit or it does not type-check
// (before the handler registry existed).
func baseline(rev string) map[uint32]*packet {
	if rev == "" {
		return nil
	}
	dir, err := os.MkdirTemp("", "packet-progress-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := exec.Command("git", "worktree", "add", "--detach", "--quiet", dir, rev).Run(); err != nil {
		log.Printf("cannot check out %s (%v), counting every handler as new", rev, err)
		return nil
	}
	defer git("worktree", "remove", "--force", dir)
	pkgs, err := load(dir, false)
	if err != nil {
		log.Printf("%s does not type-check, counting every handler as new", rev)
		return nil
	}
	return analyze(pkgs)
}

// introducedBy is the oldest commit since base touching p's handler, the one
// that added or rewrote it; HEAD when none did (the change came from code the
// handler calls).
func introducedBy(p *packet, base string) *commit {
	file, line, _ := strings.Cut(p.Pos, ":")
	if file != "" && p.End > 0 {
		span := "HEAD"
		if base != "" {
			span = base + "..HEAD"
		}
		out := git("log", "-s", "--format=%h%x00%ct%x00%s", "-L", line+","+strconv.Itoa(p.End)+":"+file, span)
		if lines := strings.Split(out, "\n"); out != "" {
			if c := parseCommit(lines[len(lines)-1]); c != nil {
				return c
			}
		}
	}
	return parseCommit(git("log", "-1", "--format=%h%x00%ct%x00%s"))
}

func parseCommit(line string) *commit {
	fields := strings.Split(line, "\x00")
	if len(fields) != 3 {
		return &commit{Summary: "Uncommitted changes", Time: time.Now()}
	}
	unix, _ := strconv.ParseInt(fields[1], 10, 64)
	return &commit{Rev: fields[0], Time: time.Unix(unix, 0), Summary: summary(fields[2])}
}

// summary turns a commit subject into a sentence: without its type(scope)
// prefix and its trailing packet ids.
func summary(subject string) string {
	if prefix, rest, ok := strings.Cut(subject, ": "); ok && !strings.Contains(prefix, " ") {
		subject = rest
	}
	if i := strings.LastIndex(subject, " (CS_"); i > 0 && strings.HasSuffix(subject, ")") {
		subject = subject[:i]
	}
	if subject == "" {
		return subject
	}
	return strings.ToUpper(subject[:1]) + subject[1:]
}

func periodLabel(d time.Duration) string {
	switch {
	case d%(24*time.Hour) == 0 && d > 24*time.Hour:
		return fmt.Sprintf("%d days", d/(24*time.Hour))
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	}
	return d.String()
}

// git runs a git command in the working directory, its trimmed output or
// nothing on failure.
func git(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

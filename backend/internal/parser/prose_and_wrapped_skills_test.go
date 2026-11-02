package parser

import (
	"strings"
	"testing"
)

const wrappedContent = `\section{Summary}
\small{
Software Engineer with hands-on experience
shipping production mobile and full-stack software across
app, API, backend, data and deployment layers.
}
\section{Technical Skills}
\small{
\textbf{Programming:} Kotlin, Java, Dart, TypeScript, Python,
SQL, Object-Oriented Programming, Data Structures, Algorithms \\

\textbf{Mobile and Web:} Flutter, Android, Jetpack Compose,
MVVM, Coroutines, Flow, Next.js, React, REST APIs, WebSockets \\

\textbf{Backend and Data:} Ktor, PostgreSQL, SQLite, Room,
Firebase, Authentication, Caching, Real-Time Systems \\

\textbf{Product Engineering:} Debugging, Root Cause Analysis,
Testing, Playwright, Code Review, CI/CD, GitHub Actions,
Docker, Vercel, Agile, Product Analytics \\

\textbf{AI-Assisted Development:} Claude Code, OpenAI Codex,
ChatGPT, GitHub Copilot
}`

func TestWrappedSummaryAndMultilineSkills(t *testing.T) {
	r := mustParse(t, wrappedContent, "")
	if len(r.Sections) != 2 {
		t.Fatalf("sections = %d", len(r.Sections))
	}
	summary := r.Sections[0].Items
	for _, item := range r.Sections[1].Items {
		if item.Skill != nil { t.Logf("skill %q: %q (%s)", item.Skill.Label.Value, item.Skill.Value.Value, item.Skill.Value.Mode) }
		if item.Opaque != nil { t.Logf("opaque span %v", item.Opaque.Span) }
	}
	if len(summary) != 1 || summary[0].Paragraph == nil || summary[0].Paragraph.Text.Mode != "text" ||
		!strings.Contains(summary[0].Paragraph.Text.Value, "app, API, backend") {
		t.Fatalf("summary: %+v", summary)
	}
	items := r.Sections[1].Items
	if len(items) != 5 {
		t.Fatalf("want 5 skill rows without read-only gaps, got %+v", items)
	}
	for i, want := range []string{"SQL, Object-Oriented", "MVVM, Coroutines", "Firebase, Authentication", "Testing, Playwright", "ChatGPT, GitHub Copilot"} {
		if items[i].Skill == nil || items[i].Skill.Value.Mode != "text" || !strings.Contains(items[i].Skill.Value.Value, want) {
			t.Errorf("skill row %d: %+v; want value containing %q", i, items[i], want)
		}
	}
}

package parser

import (
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func TestLearnRoles(t *testing.T) {
	cases := []struct {
		name       string
		count      int
		body, role string
	}{
		{`\resumeItem`, 1, `\item\small{#1}`, "bullet"},
		{`\listStart`, 0, `\begin{itemize}[leftmargin=0pt]`, "listOpen"},
		{`\listEnd`, 0, `\end{itemize}`, "listClose"},
		{`\title`, 1, `\section{#1}`, "section"},
		{`\heading`, 2, `\begin{tabular}{lr}\textbf{#1} & #2\end{tabular}`, "entry"},
		{`\skill`, 2, `\textbf{#1}{: #2}`, "skill"},
		{`\link`, 2, `\href{#1}{#2}`, "link"},
	}
	for _, tc := range cases {
		role, args := learnRole(tc.name, tc.count, tc.body)
		if role != tc.role || len(args) != tc.count {
			t.Errorf("%s: role %s, args %+v", tc.name, role, args)
		}
	}
	_, args := learnRole(`\x`, 4, `\textbf{#1} \emph{#2} \hfill #3 \vspace{#4}`)
	if args[0].Slot != model.Bold || args[1].Slot != model.Italic || args[2].Slot != model.Right || args[3].Use != "length" {
		t.Fatalf("slots: %+v", args)
	}
	_, args = learnRole(`\link`, 3, `\href{#1}{#2} #3`)
	if args[0].Use != "url" || args[1].Use != "text" || args[2].Use != "text" {
		t.Fatalf("href arguments: %+v", args)
	}
	defs := []definition{
		{CommandInfo: model.CommandInfo{Name: `\inner`, ArgCount: 1}, Body: `\item{#1}`},
		{CommandInfo: model.CommandInfo{Name: `\outer`, ArgCount: 1}, Body: `\inner{#1}`},
	}
	learnRoles(defs)
	if defs[1].Role != "bullet" {
		t.Fatalf("indirection: %+v", defs[1])
	}
}

package parser

import (
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"strings"
	"testing"
)

func mustParse(t *testing.T, body string, preamble string) *model.Resume {
	t.Helper()
	src := `\documentclass{article}` + "\n" + preamble + "\n\\begin{document}\n" + body + "\n\\end{document}\n"
	r, err := Parse(src, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A template whose commands live in a .cls file parses from the dictionary,
// with no code change and no local definitions.
func TestDictionaryModernCV(t *testing.T) {
	r := mustParse(t, `\section{Education}
\cventry{2010--2014}{BSc}{MIT}{Boston}{}{Thesis}
\cvitem{Languages}{English, French}
\cvlistitem{Open source}
\section{Skills}`, "")
	s := r.Sections[0]
	if len(r.Sections) != 2 || len(s.Items) != 3 {
		t.Fatalf("%+v", s.Items)
	}
	if s.Items[0].Entry == nil || len(s.Items[0].Entry.Fields) != 6 || s.Items[0].Entry.Fields[1].Value != "BSc" {
		t.Fatalf("cventry: %+v", s.Items[0])
	}
	if s.Items[1].Skill == nil || s.Items[1].Skill.Label.Value != "Languages" {
		t.Fatalf("cvitem: %+v", s.Items[1])
	}
	if s.Items[2].Paragraph == nil || s.Items[2].Paragraph.Text.Value != "Open source" {
		t.Fatalf("cvlistitem: %+v", s.Items[2])
	}
}

func TestDictionaryAltaCVHeader(t *testing.T) {
	r := mustParse(t, `\name{Ada Lovelace}
\personalinfo{%
  \email{ada@example.com}
  \phone{000-00-0000}
}
\cvsection{Experience}
\cvevent{Engineer}{Analytical}{1842}{London}
\begin{itemize}
\item Wrote the first program
\end{itemize}`, "")
	if r.Header.Name == nil || r.Header.Name.Value != "Ada Lovelace" || len(r.Header.Items) != 2 || r.Header.Items[0].Value != "ada@example.com" {
		t.Fatalf("header: %+v", r.Header)
	}
	if r.Report.Entries != 1 || r.Report.Bullets != 1 {
		t.Fatalf("report: %+v", r.Report)
	}
}

// A local definition beats the dictionary for the same name.
func TestLocalDefinitionWinsOverDictionary(t *testing.T) {
	r := mustParse(t, `\section{A}
\cvitem{x}{y}`, `\newcommand{\cvitem}[2]{\textbf{#1} \hfill #2}`)
	if r.Sections[0].Items[0].Entry == nil {
		t.Fatalf("local role ignored: %+v", r.Sections[0].Items[0])
	}
}

// Calls of other custom commands are expanded several levels deep.
func TestExpansionDepthAndSubstitution(t *testing.T) {
	pre := `\newcommand{\a}[1]{\item #1}
\newcommand{\b}[1]{\a{#1}}
\newcommand{\c}[1]{\b{#1}}
\newcommand{\d}[1]{\c{#1}}`
	r := mustParse(t, "\\section{S}\n\\begin{itemize}\n\\d{deep bullet}\n\\end{itemize}", pre)
	if r.Report.Bullets != 1 {
		t.Fatalf("4-level indirection: %+v", r.Report)
	}
	for _, c := range r.Meta.Commands {
		if c.Name == `\d` && c.Role != "bullet" {
			t.Fatalf("role of \\d: %s", c.Role)
		}
	}
}

// Logic the parser cannot evaluate is marked unknown, not guessed.
func TestLogicIsUnknown(t *testing.T) {
	pre := `\newcommand{\ev}[2]{\ifthenelse{\equal{#2}{}}{}{\textbf{#1} \hfill #2}}`
	r := mustParse(t, `\section{S}`+"\n"+`\ev{a}{b}`, pre)
	for _, c := range r.Meta.Commands {
		if c.Name == `\ev` && c.Role != "other" {
			t.Fatalf("conditional guessed as %s", c.Role)
		}
	}
}

func TestNestedBulletsAreChildren(t *testing.T) {
	r := mustParse(t, `\section{Experience}
\begin{itemize}
\item Top one
\begin{itemize}
\item Child a
\item Child b
\end{itemize}
\item Top two
\end{itemize}`, "")
	list := r.Sections[0].Items[0].Bullets
	if list == nil || len(list.Bullets) != 2 {
		t.Fatalf("top-level bullets: %+v", list)
	}
	top := list.Bullets[0]
	if top.Text.Value != "Top one" || len(top.Children) != 1 || len(top.Children[0].Bullets) != 2 {
		t.Fatalf("children: %+v", top)
	}
	if list.Bullets[1].Text.Value != "Top two" {
		t.Fatalf("sibling after nested list: %+v", list.Bullets[1])
	}
	if r.Report.Bullets != 4 {
		t.Fatalf("nested bullets must count in report: %+v", r.Report)
	}
	// The bullet's span covers its children; its text does not.
	if !strings.Contains(r.Sections[0].Title.Value, "Experience") || top.Span[1] <= top.Text.Span[1] {
		t.Fatalf("spans: %+v", top)
	}
}

func TestUnlabeledListIsStructure(t *testing.T) {
	pre := `\newcommand{\wrapStart}{\begin{itemize}[leftmargin=0pt, label={}]}
\newcommand{\wrapEnd}{\end{itemize}}
\newcommand{\row}[2]{\item #1 \hfill #2}
\newcommand{\it}[1]{\item #1}
\newcommand{\bs}{\begin{itemize}}
\newcommand{\be}{\end{itemize}}`
	r := mustParse(t, `\section{S}
\wrapStart
\row{Co}{2020}
\bs
\it{did x}
\be
\wrapEnd`, pre)
	if r.Report.Entries != 1 || r.Report.Bullets != 1 {
		t.Fatalf("%+v", r.Report)
	}
}

func TestHeaderBulletItemBracedForm(t *testing.T) {
	r := mustParse(t, `\section{S}
\begin{itemize}
\item{Braced form}
\item Plain form
\end{itemize}`, "")
	if r.Report.Bullets != 2 {
		t.Fatalf("%+v", r.Report)
	}
}

func TestSkillInlineColonForm(t *testing.T) {
	r := mustParse(t, `\section{Technical Skills}
\begin{itemize}[label={}]
\item{
\textbf{Languages:} Java, Python \\
\textbf{Tools:} Git, Docker
}
\end{itemize}`, "")
	n := 0
	for _, it := range r.Sections[0].Items {
		if it.Skill != nil {
			n++
			if it.Skill.Label.Value == "Languages:" && it.Skill.Value.Value != "Java, Python" {
				t.Fatalf("value %q", it.Skill.Value.Value)
			}
			if it.Skill.Label.Value == "Tools:" && it.Skill.Value.Value != "Git, Docker" {
				t.Fatalf("value %q", it.Skill.Value.Value)
			}
		}
	}
	if n != 2 {
		t.Fatalf("skills: %d", n)
	}
}

func TestSkillTabularForm(t *testing.T) {
	r := mustParse(t, `\section{Skills}
\begin{tabular}{ll}
\textbf{Languages} & Go, Rust \\
Tools & Git, Docker \\
\end{tabular}`, "")
	var got []string
	for _, it := range r.Sections[0].Items {
		if it.Skill != nil {
			got = append(got, it.Skill.Label.Value+"="+it.Skill.Value.Value)
		}
	}
	if strings.Join(got, ";") != "**Languages**=Go, Rust;Tools=Git, Docker" {
		t.Fatalf("%q", got)
	}
}

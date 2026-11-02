package parser

import (
	"testing"
)

func TestMultilineSkillsListDoesNotLeaveUnsupportedRegion(t *testing.T) {
	r := mustParse(t, `\section{Technical Skills}
\begin{itemize}[
    leftmargin=0pt,
    label={},
    itemsep=2pt,
    topsep=2pt
]
    \small{
        \item{
            \textbf{Languages:} Go, TypeScript, Python, C++ \\
            \textbf{Backend \& Databases:}
            FastAPI, PostgreSQL, MySQL, MongoDB, Redis, Kafka \\
            \textbf{Cloud \& AI:}
            LangGraph, Docker, AWS (IAM, S3, EC2)
        }
    }
\end{itemize}`, "")
	var skills, opaque int
	for _, item := range r.Sections[0].Items {
		if item.Skill != nil {
			skills++
		}
		if item.Opaque != nil {
			opaque++
		}
	}
	if skills != 3 || opaque != 0 {
		t.Fatalf("want 3 skills and no unsupported region; got skills=%d opaque=%d items=%+v", skills, opaque, r.Sections[0].Items)
	}
}

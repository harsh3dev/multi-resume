/**
 * Mini-markdown <-> rich-text document conversion.
 *
 * The backend (backend/internal/textconv) is the source of truth for the
 * grammar: `**bold**`, `*italic*`, `[text](url)`, with `\*`, `\[`, `\]` as
 * literal escapes. `parse` mirrors textconv.encode's scanning rules exactly
 * (first matching closing marker, unmatched markers are literal), so the
 * editor shows what LaTeX will render. Paragraphs are joined by "\n".
 *
 * The document shape is a minimal subset of TipTap's JSON.
 */

export type Mark = { type: "bold" } | { type: "italic" } | { type: "link"; attrs: { href: string } };
export type TextNode = { type: "text"; text: string; marks?: Mark[] };
export type ProtectedNode = { type: "protectedLatex"; attrs: { source: string }; marks?: Mark[] };
export type ParagraphNode = { type: "paragraph"; content?: (TextNode | ProtectedNode)[] };
export type DocNode = { type: "doc"; content: ParagraphNode[] };

/** Same rule as the backend's allowedURL, plus the chars the grammar can't carry. */
export function isAllowedUrl(url: string): boolean {
  if (!/^(https:\/\/|http:\/\/|mailto:)/.test(url)) return false;
  return !/[{}\\()\s\[\]]/.test(url);
}

function marked(value: string, start: number, mark: string): [string, number] | null {
  const end = value.indexOf(mark, start);
  if (end - start <= 0) return null;
  return [value.slice(start, end), end + mark.length];
}

function parseInline(value: string, marks: Mark[]): (TextNode | ProtectedNode)[] {
  const out: (TextNode | ProtectedNode)[] = [];
  let buf = "";
  const flush = () => {
    if (buf) out.push(marks.length ? { type: "text", text: buf, marks } : { type: "text", text: buf });
    buf = "";
  };

  for (let i = 0; i < value.length; ) {
    const ch = value[i];
    if (ch === "⟦") {
      const end = value.indexOf("⟧", i + 1);
      if (end > i + 1) {
        flush();
        out.push({ type: "protectedLatex", attrs: { source: value.slice(i + 1, end) }, ...(marks.length ? { marks } : {}) });
        i = end + 1;
        continue;
      }
    }
    if (ch === "\\" && i + 1 < value.length && "*[]".includes(value[i + 1])) {
      buf += value[i + 1];
      i += 2;
    } else if (ch === "*") {
      const bold = value.startsWith("**", i);
      const found = bold ? marked(value, i + 2, "**") : marked(value, i + 1, "*");
      if (!found) {
        buf += "*";
        i++;
        continue;
      }
      flush();
      out.push(...parseInline(found[0], [...marks, { type: bold ? "bold" : "italic" }]));
      i = found[1];
    } else if (ch === "[") {
      const close = value.indexOf("]", i);
      if (close < 0 || value[close + 1] !== "(") {
        buf += "[";
        i++;
        continue;
      }
      const paren = value.indexOf(")", close + 2);
      if (paren < 0) {
        buf += "[";
        i++;
        continue;
      }
      flush();
      const href = value.slice(close + 2, paren);
      out.push(...parseInline(value.slice(i + 1, close), [...marks, { type: "link", attrs: { href } }]));
      i = paren + 1;
    } else {
      buf += ch;
      i++;
    }
  }
  flush();
  return out;
}

export function parse(value: string): DocNode {
  return {
    type: "doc",
    content: value.split("\n").map((line) => {
      const content = parseInline(line, []);
      return content.length ? { type: "paragraph", content } : { type: "paragraph" };
    }),
  };
}

function escapeText(text: string, inLink: boolean): string {
  // `[`/`]` can't be carried inside a link label (the backend closes the
  // label at the first `]`), so they are dropped there.
  if (inLink) return text.replace(/[[\]]/g, "").replace(/\*/g, "\\*");
  return text.replace(/[*[\]]/g, (c) => `\\${c}`);
}

export function serialize(doc: { content?: ParagraphNode[] }): string {
  return (doc.content ?? [])
    .map((p) => {
      let result = "";
      let active: Mark[] = [];
      const close = (marks: Mark[]) => {
        for (const mark of [...marks].reverse()) {
          result += mark.type === "bold" ? "**" : mark.type === "italic" ? "*" : `](${mark.attrs.href})`;
        }
      };
      const open = (marks: Mark[]) => {
        for (const mark of marks) {
          result += mark.type === "bold" ? "**" : mark.type === "italic" ? "*" : "[";
        }
      };
      for (const node of p.content ?? []) {
        const marks = [...(node.marks ?? [])].sort((a, b) =>
          ({ link: 0, bold: 1, italic: 2 })[a.type] - ({ link: 0, bold: 1, italic: 2 })[b.type],
        );
        const shared = active.findIndex((mark, i) => JSON.stringify(mark) !== JSON.stringify(marks[i]));
        const common = shared < 0 ? Math.min(active.length, marks.length) : shared;
        close(active.slice(common));
        open(marks.slice(common));
        active = marks;
        result += node.type === "protectedLatex"
          ? `⟦${node.attrs.source}⟧`
          : escapeText(node.text, marks.some((mark) => mark.type === "link"));
      }
      close(active);
      return result;
    })
    .join("\n");
}

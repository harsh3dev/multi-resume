"use client";

import Link from "@tiptap/extension-link";
import { Extension, Node as TiptapNode, type JSONContent } from "@tiptap/core";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { EditorContent, useEditor, useEditorState } from "@tiptap/react";
import { BubbleMenu } from "@tiptap/react/menus";
import StarterKit from "@tiptap/starter-kit";
import { CloseCircle, Link21, TextBold, TextItalic, TickCircle } from "@/icons";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";

import { IconButton } from "@/components/ui/components/actions/icon-button";
import { Input } from "@/components/ui/components/forms/input";
import { isAllowedUrl, parse, serialize, type DocNode } from "@/lib/utils/mini-markdown";
import { cn } from "@/lib/utils/utils";

/**
 * The mini-markdown grammar can't express bold+italic on the same text
 * (`***x***` is ambiguous to the backend), so the two marks are exclusive.
 */
const ExclusiveMarks = Extension.create({
  name: "exclusiveMarks",
  priority: 1000,
  addKeyboardShortcuts() {
    return {
      "Mod-b": () => this.editor.chain().unsetItalic().toggleBold().run(),
      "Mod-i": () => this.editor.chain().unsetBold().toggleItalic().run(),
    };
  },
});

/** Single-line fields swallow Enter instead of splitting the paragraph. */
const SingleLine = Extension.create({
  name: "singleLine",
  priority: 1000,
  addKeyboardShortcuts() {
    return { Enter: () => true };
  },
});

// Unknown inline LaTeX is an atomic, non-editable chip. The server requires
// every original token to survive in order; block deletions/duplicates here
// instead of sending an edit that the server must reject.
const ProtectedLatex = TiptapNode.create({
  name: "protectedLatex",
  group: "inline",
  inline: true,
  atom: true,
  selectable: false,
  addAttributes() {
    return { source: { default: "" } };
  },
  parseHTML() {
    return [{ tag: "span[data-protected-latex]", getAttrs: (element) => ({ source: (element as HTMLElement).dataset.protectedLatex ?? "" }) }];
  },
  renderHTML({ node }) {
    return ["span", {
      "data-protected-latex": node.attrs.source,
      contenteditable: "false",
      title: "Protected LaTeX — edit in the Source tab",
      class: "rounded border border-border bg-muted px-1 font-mono text-xs text-muted-foreground",
    }, node.attrs.source];
  },
  addProseMirrorPlugins() {
    const tokens = (doc: typeof this.editor.state.doc) => {
      const found: string[] = [];
      doc.descendants((node) => {
        if (node.type.name === "protectedLatex") found.push(node.attrs.source);
      });
      return found;
    };
    return [new Plugin({
      filterTransaction(tr, state) {
        if (!tr.docChanged) return true;
        const before = tokens(state.doc);
        const after = tokens(tr.doc);
        if (before.length !== after.length || before.some((token, i) => token !== after[i])) return false;
        // Text outside the chip is editable. Do not let a pasted literal
        // ⟦…⟧ become a newly invented protected command on the server.
        const oldText = serialize(state.doc.toJSON() as DocNode);
        const newText = serialize(tr.doc.toJSON() as DocNode);
        const tokenPattern = /⟦[^⟦⟧]*⟧/g;
        const literalTokens = (text: string) => text.match(tokenPattern) ?? [];
        return JSON.stringify(literalTokens(oldText)) === JSON.stringify(literalTokens(newText));
      },
    })];
  },
});

function buildExtensions(multiline: boolean) {
  return [
    StarterKit.configure({
      // Only what the mini-markdown grammar can express.
      heading: false,
      blockquote: false,
      bulletList: false,
      orderedList: false,
      listItem: false,
      listKeymap: false,
      codeBlock: false,
      code: false,
      strike: false,
      underline: false,
      horizontalRule: false,
      hardBreak: false,
      link: false,
    }),
    Link.configure({
      openOnClick: false,
      autolink: false,
      linkOnPaste: false,
      defaultProtocol: "https",
      isAllowedUri: (url) => isAllowedUrl(url),
      HTMLAttributes: { rel: "noopener noreferrer nofollow", class: "text-primary underline underline-offset-2" },
    }),
    ExclusiveMarks,
    ProtectedLatex,
    ...(multiline ? [] : [SingleLine]),
  ];
}

const MOD = typeof navigator !== "undefined" && /Mac/.test(navigator.platform) ? "⌘" : "Ctrl+";
const richTextMenuKey = new PluginKey("richTextBubbleMenu");

export function RichText({
  value,
  onChange,
  multiline = false,
  label,
  className,
  variant = "box",
}: {
  /** Mini-markdown. The store keeps this string; TipTap is only a view. */
  value: string;
  onChange: (value: string) => void;
  multiline?: boolean;
  label: string;
  className?: string;
  /**
   * "box": a bordered input well. "bare": no border/fill/ring of its own,
   * for use inside a container that draws the single outline (see BulletItem).
   */
  variant?: "box" | "bare";
}) {
  const [linkOpen, setLinkOpen] = useState(false);
  const [href, setHref] = useState("https://");
  const [linkError, setLinkError] = useState<string | null>(null);
  const [editingLink, setEditingLink] = useState(false);
  const [popStyle, setPopStyle] = useState<CSSProperties>({});
  const rootRef = useRef<HTMLDivElement>(null);
  const formRef = useRef<HTMLFormElement>(null);
  const menuDismissed = useRef(false);
  const onChangeRef = useRef(onChange);
  const valueRef = useRef(value);
  onChangeRef.current = onChange;
  valueRef.current = value;

  const editor = useEditor({
    immediatelyRender: false,
    extensions: buildExtensions(multiline),
    content: parse(value) as JSONContent,
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-label": label,
        "aria-multiline": String(multiline),
        class: "outline-none",
      },
    },
    onUpdate: ({ editor: ed }) => {
      const md = serialize(ed.getJSON() as DocNode);
      // Only user edits reach here, so an untouched value is never rewritten.
      if (md !== valueRef.current) onChangeRef.current(md);
    },
    onSelectionUpdate: () => {
      // A new selection (mouse or keyboard) is a new request for the toolbar.
      menuDismissed.current = false;
    },
  });

  useEffect(() => {
    if (!linkOpen) return;
    const openedAt = Date.now();
    // Ignore the scrolls caused by opening/focusing the form itself.
    const close = (e?: Event) => {
      if (Date.now() - openedAt < 300) return;
      if (e?.target instanceof Node && formRef.current?.contains(e.target)) return;
      setLinkOpen(false);
    };
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close, true);
    };
  }, [linkOpen]);

  // Reconcile server-driven changes (revision refetch, "edit from here").
  // Skipped while focused: the server normalises text (e.g. `--` → en dash)
  // and replacing content mid-typing would move the caret.
  function reconcile() {
    if (!editor || editor.isDestroyed) return;
    if (serialize(editor.getJSON() as DocNode) !== value) {
      editor.commands.setContent(parse(value) as JSONContent, { emitUpdate: false });
    }
  }
  useEffect(() => {
    if (editor && !editor.isFocused) reconcile();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, editor]);

  // v3 doesn't re-render per transaction; subscribe only to the mark state.
  const active = useEditorState({
    editor,
    selector: ({ editor: ed }) => ({
      bold: ed?.isActive("bold") ?? false,
      italic: ed?.isActive("italic") ?? false,
      link: ed?.isActive("link") ?? false,
    }),
  });

  if (!editor) {
    return <div className={cn("h-8 rounded-lg border border-input bg-background", className)} aria-busy />;
  }

  function openLink() {
    if (!editor) return;
    dismissMenu();
    // On an existing link the form edits it (prefilled); otherwise it adds one.
    const existing = editor.isActive("link")
      ? (editor.getAttributes("link").href as string | undefined)
      : undefined;
    setEditingLink(existing !== undefined);
    setHref(existing ?? "https://");
    setLinkError(null);
    placePopover();
    setLinkOpen(true);
  }

  function dismissMenu() {
    menuDismissed.current = true;
    if (editor && !editor.isDestroyed) {
      editor.view.dispatch(editor.state.tr.setMeta(richTextMenuKey, "hide"));
    }
  }

  // The popover is portalled to <body> (panes clip overflow) and clamped to
  // the viewport; it flips above the field when there is no room below.
  function placePopover() {
    const rect = rootRef.current?.getBoundingClientRect();
    if (!rect) return;
    const margin = 12;
    const width = Math.min(384, window.innerWidth - margin * 2);
    const left = Math.min(Math.max(rect.left, margin), window.innerWidth - width - margin);
    const below = window.innerHeight - rect.bottom > 120;
    setPopStyle(
      below
        ? { left, width, top: rect.bottom + 4 }
        : { left, width, bottom: window.innerHeight - rect.top + 4 },
    );
  }

  function removeLink() {
    if (!editor) return;
    editor.chain().focus().extendMarkRange("link").unsetLink().run();
    setLinkOpen(false);
  }

  function applyLink() {
    if (!editor) return;
    const url = href.trim();
    if (!isAllowedUrl(url)) {
      setLinkError("Use an http(s) or mailto link without spaces or brackets.");
      return;
    }
    const chain = editor.chain().focus();
    if (editingLink) {
      chain.extendMarkRange("link").setLink({ href: url }).run();
    } else if (editor.state.selection.empty) {
      chain.insertContent({ type: "text", text: url, marks: [{ type: "link", attrs: { href: url } }] }).run();
    } else {
      chain.setLink({ href: url }).run();
    }
    setLinkOpen(false);
  }

  return (
    <div
      ref={rootRef}
      className={cn("group/rich relative flex flex-col gap-1", className)}
      onBlur={(e) => {
        // The form is portalled, and its autoFocus can fire before formRef is
        // attached, so recognise it by attribute instead of by ref.
        const next = e.relatedTarget as Element | null;
        if (!e.currentTarget.contains(next) && !next?.closest?.("[data-rich-link-form]")) {
          setLinkOpen(false);
          reconcile();
        }
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          dismissMenu();
          setLinkOpen(false);
          return;
        }
        if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
          e.preventDefault();
          openLink();
        }
      }}
      onPointerDownCapture={(e) => {
        if (e.target instanceof Element && e.target.closest('[role="textbox"]')) {
          menuDismissed.current = false;
        }
      }}
    >
      <EditorContent
        editor={editor}
        className={cn(
          // Long text scrolls inside the field instead of growing without limit.
          "max-h-60 w-full overflow-y-auto text-sm",
          variant === "box" &&
            "rounded-lg border border-field-border bg-field px-2.5 py-1.5 transition-colors focus-within:border-foreground/25 focus-within:bg-field-focus focus-within:inset-shadow-track",
          variant === "bare" && "px-1 py-0.5",
          multiline && variant === "box" && "min-h-16",
        )}
      />
      {/* Floating toolbar: only while text is selected, never on plain focus. */}
      <BubbleMenu
        editor={editor}
        pluginKey={richTextMenuKey}
        updateDelay={0}
        options={{ placement: "top", offset: 8 }}
        shouldShow={({ editor: ed, element, view, state }) =>
          !menuDismissed.current &&
          !linkOpen &&
          (view.hasFocus() || element.contains(document.activeElement)) &&
          (!state.selection.empty || ed.isActive("link"))
        }
        className="z-50 flex items-center gap-0.5 rounded-lg border border-border bg-popover p-0.5 text-popover-foreground shadow-md"
      >
        <IconButton
          icon={TextBold}
          label="Bold"
          shortcut={` ${MOD}B`}
          size="icon-xs"
          aria-pressed={active?.bold}
          className="aria-pressed:bg-muted"
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => editor.chain().focus().unsetItalic().toggleBold().run()}
        />
        <IconButton
          icon={TextItalic}
          label="Italic"
          shortcut={` ${MOD}I`}
          size="icon-xs"
          aria-pressed={active?.italic}
          className="aria-pressed:bg-muted"
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => editor.chain().focus().unsetBold().toggleItalic().run()}
        />
        <IconButton
          icon={Link21}
          label={active?.link ? "Edit link" : "Add link"}
          shortcut={` ${MOD}K`}
          size="icon-xs"
          aria-pressed={active?.link}
          className="aria-pressed:bg-muted"
          onMouseDown={(e) => e.preventDefault()}
          onClick={openLink}
        />
      </BubbleMenu>
      {linkOpen
        ? createPortal(
        <form
          ref={formRef}
          data-rich-link-form=""
          style={popStyle}
          className="fixed z-50 flex flex-col gap-1.5 rounded-xl border border-border bg-popover p-2 text-popover-foreground shadow-md"
          onSubmit={(e) => {
            e.preventDefault();
            applyLink();
          }}
        >
          <div className="flex items-center gap-1">
            <Input
              autoFocus
              aria-label="Link URL"
              aria-invalid={linkError ? true : undefined}
              value={href}
              onChange={(e) => {
                setHref(e.target.value);
                setLinkError(null);
              }}
              onKeyDown={(e) => {
                if (e.key === "Escape") {
                  e.stopPropagation();
                  setLinkOpen(false);
                  editor.commands.focus();
                }
              }}
              className="h-9 min-w-0 flex-1 text-sm"
            />
            <IconButton
              type="submit"
              icon={TickCircle}
              label={editingLink ? "Update link" : "Apply link"}
              size="icon"
              className="text-success"
            />
            <IconButton
              type="button"
              icon={CloseCircle}
              label={editingLink ? "Remove link" : "Cancel"}
              size="icon"
              className="text-destructive"
              onClick={editingLink ? removeLink : () => setLinkOpen(false)}
            />
          </div>
          {linkError ? <p className="text-xs text-destructive">{linkError}</p> : null}
        </form>,
        document.body,
      )
        : null}
    </div>
  );
}

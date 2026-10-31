# AGENTS.md

## Design token contract (non-negotiable)

The design system lives in `apps/web/src/app/globals.css` and nowhere else.

1. **No hardcoded colors.** Never use raw Tailwind palette classes (`text-red-700`, `bg-amber-100`, …), hex/oklch literals, or `canvasBackground="#fff"` in app code. Only semantic token classes: `bg-background`, `text-foreground`, `text-muted-foreground`, `text-destructive`, `text-success`, `text-warning`, `border-border`, `bg-sidebar`, etc.
2. **Everything is a token.** Colors, radii (`rounded-lg` → `--radius`), shadows (`shadow-md` → `--shadow-md`), tracking, spacing and fonts all resolve to CSS variables mapped via `@theme inline`. To change the design system, edit `globals.css` only.
3. **Both themes, always.** Light and dark values are defined in `:root` / `.dark`. Theme switching is `next-themes` (class strategy, system default). Review UI changes in both.
4. **Guard:** `bun run lint:tokens` (in `apps/web`) must pass — it rejects raw palette classes and hex literals in `src/`.
5. **Icons:** Iconsax Free via `iconsax-react`, always through `@/components/ui/icon` (`AppIcon`), which locks the size scale and the style: **Linear** (default) for controls and secondary icons — icon buttons, chevrons, drag handles, menu arrows — and **Bulk** (`variant="Bulk"`) for feature icons: nav, logo/title tiles, empty states, dialog tiles, toasts, status. Do not add another icon library.
6. **Fonts — which font where (non-negotiable).** Three roles, loaded in `src/app/layout.tsx` with the variables on `<html>` (not `<body>`). Use the classes only; never `font-family`, `fontFamily`, `font-[...]`, or another font package.

   | Role | Class | Font | Use for | Never for |
   |---|---|---|---|---|
   | UI (default) | `font-sans` (inherited from `html`, don't repeat it) | Geist | Everything by default: body text, buttons, inputs, tabs, menus, dialogs, toasts, tables, labels, helper text, the rich-text editor, the PDF toolbar, sidebar nav | — |
   | Display | `font-serif` | Instrument Serif | **Only** the brand wordmark ("Resume Editor") and page-level `h1` titles (landing hero, login, dashboard, settings). One per screen, `text-lg` and up. | Body copy, `h2`+ section headings, card/dialog titles, buttons, form labels, anything below `text-lg` |
   | Technical | `font-mono` | Geist Mono | Code and verbatim source: LaTeX/`.tex`, raw-mode (read-only) fields, `<pre>`, the Source tab, IDs, file names | Prose, headings, numbers in normal UI (use `tabular-nums` instead) |

   Rule of thumb: if it is something the user reads or clicks, it is `font-sans`; serif is for a title you would put on a poster; mono is for text that must be shown exactly as written. Changing a font means editing `layout.tsx` / `globals.css` only. Guard: `bun run lint:fonts`.

Spec: `# Resume Editor — UX Flow & Design Syste.md` (repo root). Revamp plan: `.docs/tasks/phase-10-frontend-revamp.md`.

## App shell layout

`src/components/layout/app-shell.tsx` follows the invoicely pattern: the **sidebar colour is the base layer** (`bg-sidebar`) and the main area is a **raised panel** (`bg-background`, `rounded-xl`, border, `shadow-sm`) on top of it. The sidebar collapses to an icon rail (⌘/Ctrl+\, state in the `sidebar_state` cookie, read server-side to avoid a flash). There is **one header bar** per screen: pages put their title/actions into it with `<PageHeader>` (portal into the shell's slot) and use `<PageTitle>` (gradient icon tile + serif title) as the page h1; do not add a second page-level bar.

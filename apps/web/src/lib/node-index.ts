import type { Field, Resume } from "@/types/generated";

/** Where a node sits in the resume, and its current text (for display). */
export interface NodeInfo {
  /** Section title, or "Header". */
  section: string;
  /** The entry/skill it belongs to, e.g. the company a bullet is under. */
  context?: string;
  /** Current text of the node, if it has any. */
  text?: string;
}

const plain = (s: string) => s.replace(/\*+/g, "").replace(/\\([*[\]])/g, "$1").trim();
const firstText = (fields: Field[]) => fields.map((f) => plain(f.value)).find((v) => v.length > 0);

/**
 * Maps every editable id in the tree (header fields, section ids and titles,
 * entries and their fields, bullet lists and bullets, skills, paragraphs) to
 * where it lives. Built from the tree itself, so it works for any template.
 */
export function buildNodeIndex(tree: Resume | null): Map<string, NodeInfo> {
  const index = new Map<string, NodeInfo>();
  if (!tree) return index;

  if (tree.header) {
    const info = { section: "Header" };
    index.set(tree.header.id, info);
    if (tree.header.name) index.set(tree.header.name.id, { ...info, text: plain(tree.header.name.value) });
    for (const f of tree.header.items) index.set(f.id, { ...info, text: plain(f.value) });
  }

  for (const section of tree.sections) {
    const name = plain(section.title.value) || "Untitled section";
    index.set(section.id, { section: name, text: name });
    index.set(section.title.id, { section: name, text: name });

    for (const item of section.items) {
      switch (item.kind) {
        case "bullets":
          index.set(item.id, { section: name });
          for (const b of item.bullets) index.set(b.id, { section: name, text: plain(b.text.value) });
          break;
        case "entry": {
          const context = firstText(item.fields);
          index.set(item.id, { section: name, context, text: context });
          for (const f of item.fields) index.set(f.id, { section: name, context, text: plain(f.value) });
          if (item.bullets) {
            index.set(item.bullets.id, { section: name, context });
            for (const b of item.bullets.bullets) {
              index.set(b.id, { section: name, context, text: plain(b.text.value) });
            }
          }
          break;
        }
        case "skill": {
          const label = plain(item.label.value);
          const text = `${label}: ${plain(item.value.value)}`;
          index.set(item.id, { section: name, context: label, text });
          index.set(item.label.id, { section: name, context: label, text: label });
          index.set(item.value.id, { section: name, context: label, text: plain(item.value.value) });
          break;
        }
        case "paragraph":
          index.set(item.id, { section: name, text: plain(item.text.value) });
          index.set(item.text.id, { section: name, text: plain(item.text.value) });
          break;
        default:
          break; // opaque regions have nothing editable
      }
    }
  }
  return index;
}

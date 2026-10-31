import type { Operation, Resume } from "@/types/generated";

/**
 * Applies the visible effect of an operation to the local tree so the UI can
 * move immediately, before the server confirms. Only structural reorders are
 * mirrored (text edits already live in the field being typed in); the server's
 * tree replaces this one on success, and a failure rolls back to the last
 * confirmed tree.
 */
export function applyOptimistic(tree: Resume, op: Operation): Resume {
  if (op.op !== "reorder") return tree;
  const rank = new Map(op.order.map((id, i) => [id, i] as const));
  // Stable: anything not named in `order` keeps its relative position, last.
  const sorted = <T extends { id: string }>(items: T[]): T[] =>
    [...items].sort((a, b) => (rank.get(a.id) ?? Infinity) - (rank.get(b.id) ?? Infinity));

  if (op.parentId === "root") {
    return { ...tree, sections: sorted(tree.sections) };
  }

  return {
    ...tree,
    sections: tree.sections.map((section) => {
      const items = section.id === op.parentId ? sorted(section.items) : section.items;
      return {
        ...section,
        items: items.map((item) => {
          if (item.kind === "bullets" && item.id === op.parentId) {
            return { ...item, bullets: sorted(item.bullets) };
          }
          if (item.kind === "entry" && item.bullets && item.bullets.id === op.parentId) {
            return { ...item, bullets: { ...item.bullets, bullets: sorted(item.bullets.bullets) } };
          }
          return item;
        }),
      };
    }),
  };
}

/** Re-applies still-unsent ops on top of a (server-confirmed) tree. */
export function applyOptimisticAll(tree: Resume, ops: Iterable<Operation>): Resume {
  let next = tree;
  for (const op of ops) next = applyOptimistic(next, op);
  return next;
}

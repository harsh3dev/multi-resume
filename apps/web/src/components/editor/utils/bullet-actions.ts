import { Eye, EyeSlash, Trash } from "@/icons";

import type { IconsaxIcon } from "@/icons";
import type { Bullet } from "@/types/generated";

/**
 * One action in a bullet's "chin" (the row that opens under a focused
 * bullet). Actions are plain data: to add one, add an entry to
 * `getBulletActions` — the chin renders whatever it returns.
 */
export interface BulletAction {
  id: string;
  /** Accessible name and tooltip. */
  label: string;
  icon: IconsaxIcon;
  /** Which end of the chin the button sits at. */
  align: "left" | "right";
  /** Called when the button is pressed. */
  run: () => void;
  /** "destructive" colours the icon as a dangerous action. */
  tone?: "default" | "destructive";
}

/** What a bullet's actions may call; supplied by whoever owns the ops. */
export interface BulletActionHandlers {
  hide: () => void;
  remove: () => void;
}

export function getBulletActions(bullet: Bullet, handlers: BulletActionHandlers): BulletAction[] {
  return [
    {
      id: "toggle-hidden",
      label: bullet.hidden ? "Show bullet" : "Hide bullet",
      icon: bullet.hidden ? Eye : EyeSlash,
      align: "right",
      run: handlers.hide,
    },
    {
      id: "delete",
      label: "Delete bullet",
      icon: Trash,
      align: "right",
      tone: "destructive",
      run: handlers.remove,
    },
  ];
}

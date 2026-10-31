import type { Icon as IconsaxIcon, IconProps } from "iconsax-react";

import { cn } from "@/lib/utils";

/**
 * Application icon wrapper around iconsax-react.
 * Locks the Iconsax variant so the app uses one icon style everywhere, and
 * maps semantic sizes onto the spec's scale: 16px buttons, 18px nav, 20px
 * headers/dialogs. Never render an Iconsax component directly in app code.
 */
/**
 * Two styles, by role. Controls and secondary icons (icon buttons, chevrons,
 * drag handles, menu arrows) are Linear — the default. Feature icons (nav,
 * logo/title tiles, empty states, dialog tiles, toasts, status) pass
 * variant="Bulk".
 */
type IconVariant = "Linear" | "Bulk";

const ICON_SIZES = {
  sm: 16,
  md: 18,
  lg: 20,
} as const;

interface AppIconProps extends Omit<IconProps, "variant" | "size" | "color" | "icon"> {
  icon: IconsaxIcon;
  variant?: IconVariant;
  size?: keyof typeof ICON_SIZES | number;
}

export function AppIcon({ icon: IconComponent, size = "sm", variant = "Linear", className, ...props }: AppIconProps) {
  const px = typeof size === "number" ? size : ICON_SIZES[size];
  return (
    <IconComponent
      size={px}
      color="currentColor"
      variant={variant}
      className={cn("shrink-0", className)}
      {...props}
    />
  );
}

export type { IconsaxIcon };

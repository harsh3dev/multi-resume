"use client";

import { Notification } from "iconsax-react";

import { Button } from "@/components/ui/button";
import { AppIcon } from "@/components/ui/icon";
import { ToastOrigin } from "@/components/ui/toaster";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { toast } from "@/lib/store/toast";

/**
 * The shell's notification bell. It is the anchor toasts emerge from on every
 * page, so it lives in the shell header rather than in any one page's header.
 */
export function NotificationBell() {
  return (
    <ToastOrigin>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon"
              aria-label="Notifications"
              onClick={() => toast.info("No new notifications")}
            >
              <AppIcon icon={Notification} />
            </Button>
          }
        />
        <TooltipContent>Notifications</TooltipContent>
      </Tooltip>
    </ToastOrigin>
  );
}

"use client";

import { Notification } from "@/icons";

import { Button } from "@/components/ui/components/actions/button";
import { AppIcon } from "@/icons";
import { ToastOrigin } from "@/components/ui/components/display/toaster";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/components/actions/tooltip";
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

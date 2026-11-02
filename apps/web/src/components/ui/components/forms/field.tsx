import * as React from "react";

import { Badge } from "@/components/ui/components/display/badge";
import { Label } from "@/components/ui/components/forms/label";
import { cn } from "@/lib/utils/utils";

/**
 * Form field anatomy shared by every control:
 *   1. text-xs label (+ "Optional" badge, destructive when invalid)
 *   2. the control (single child; gets id + aria-invalid injected)
 *   3. one helper line — the error always wins over the description
 */
interface FieldProps {
  label: string;
  htmlFor?: string;
  optional?: boolean;
  error?: string;
  description?: string;
  className?: string;
  children: React.ReactElement;
}

function Field({ label, htmlFor, optional, error, description, className, children }: FieldProps) {
  const id = React.useId();
  const controlId = htmlFor ?? id;
  const control = React.cloneElement(
    children as React.ReactElement<Record<string, unknown>>,
    {
      id: controlId,
      "aria-invalid": error ? true : undefined,
    },
  );

  return (
    <div data-slot="field" className={cn("flex flex-col gap-1.5", className)}>
      <div className="flex items-center gap-1.5">
        <Label htmlFor={controlId} className="text-xs">
          {label}
        </Label>
        {optional ? (
          <Badge variant={error ? "destructive" : "secondary"} className="text-[10px]">
            Optional
          </Badge>
        ) : null}
      </div>
      {control}
      {error ? (
        <p className="text-xs text-destructive">{error}</p>
      ) : description ? (
        <p className="text-xs text-muted-foreground">{description}</p>
      ) : null}
    </div>
  );
}

export { Field };

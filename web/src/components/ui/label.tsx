import * as React from "react";
import * as LabelPrimitive from "@radix-ui/react-label";
import { cn } from "@/lib/utils";

export const Label = React.forwardRef<
  React.ElementRef<typeof LabelPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof LabelPrimitive.Root> & {
    hint?: React.ReactNode;
  }
>(({ className, children, hint, ...props }, ref) => (
  <LabelPrimitive.Root
    ref={ref}
    className={cn(
      "text-[12px] font-medium leading-none block mb-1 text-foreground",
      className,
    )}
    {...props}
  >
    <span>{children}</span>
    {hint && (
      <span className="text-muted-foreground font-normal ml-1.5">{hint}</span>
    )}
  </LabelPrimitive.Root>
));
Label.displayName = LabelPrimitive.Root.displayName;

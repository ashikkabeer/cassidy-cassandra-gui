import * as React from "react";
import { cn } from "@/lib/utils";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  icon?: React.ReactNode;
  suffix?: React.ReactNode;
  error?: boolean;
  wrapperClassName?: string;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, icon, suffix, error, wrapperClassName, ...props }, ref) => {
    return (
      <label
        className={cn(
          "flex items-center gap-1.5 px-2 h-7 w-full bg-background border border-[hsl(var(--border-strong))] rounded-[var(--radius)] transition-colors focus-within:border-[hsl(var(--ring))] focus-within:shadow-[0_0_0_3px_hsl(var(--ring)/0.18)]",
          error &&
            "border-destructive focus-within:shadow-[0_0_0_3px_hsl(var(--destructive)/0.2)]",
          wrapperClassName,
        )}
      >
        {icon && <span className="text-muted-foreground flex">{icon}</span>}
        <input
          ref={ref}
          className={cn(
            "flex-1 min-w-0 bg-transparent outline-none border-0 p-0 text-[12.5px] placeholder:text-muted-foreground",
            className,
          )}
          {...props}
        />
        {suffix && <span className="text-muted-foreground flex">{suffix}</span>}
      </label>
    );
  },
);
Input.displayName = "Input";

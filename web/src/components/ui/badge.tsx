import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center gap-1 h-[18px] px-1.5 rounded-full text-[10.5px] font-medium leading-none border whitespace-nowrap",
  {
    variants: {
      variant: {
        secondary: "bg-secondary text-secondary-foreground border-transparent",
        outline:
          "bg-transparent border-[hsl(var(--border-strong))] text-muted-foreground",
        success:
          "bg-[hsl(var(--success)/0.15)] text-[hsl(var(--success))] border-[hsl(var(--success)/0.3)]",
        warning:
          "bg-[hsl(var(--warning)/0.15)] text-[hsl(var(--warning))] border-[hsl(var(--warning)/0.3)]",
        info: "bg-[hsl(var(--info)/0.15)] text-[hsl(var(--info))] border-[hsl(var(--info)/0.3)]",
        destructive:
          "bg-[hsl(var(--destructive)/0.15)] text-[hsl(0_80%_75%)] border-[hsl(var(--destructive)/0.4)]",
        mono: "font-mono text-[10px] bg-secondary text-secondary-foreground border-transparent",
      },
    },
    defaultVariants: { variant: "secondary" },
  },
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {
  icon?: React.ReactNode;
}

export function Badge({ className, variant, icon, children, ...props }: BadgeProps) {
  return (
    <span className={cn(badgeVariants({ variant }), className)} {...props}>
      {icon && <span className="flex">{icon}</span>}
      {children}
    </span>
  );
}

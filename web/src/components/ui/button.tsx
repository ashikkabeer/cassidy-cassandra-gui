import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-[var(--radius)] text-[12.5px] font-medium leading-none transition-colors focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50 border border-transparent select-none",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground border-primary hover:bg-[hsl(0_0%_90%)]",
        secondary:
          "bg-secondary text-secondary-foreground hover:bg-[hsl(240_4%_20%)]",
        outline:
          "bg-transparent border-[hsl(var(--border-strong))] text-foreground hover:bg-accent",
        ghost: "bg-transparent text-foreground hover:bg-accent",
        destructive:
          "bg-destructive text-destructive-foreground hover:bg-[hsl(0_62%_45%)]",
        "destructive-outline":
          "bg-transparent border-[hsl(var(--destructive)/0.5)] text-[hsl(0_80%_70%)] hover:bg-[hsl(var(--destructive)/0.12)] hover:border-[hsl(var(--destructive)/0.8)]",
        link: "bg-transparent text-foreground underline underline-offset-[3px] p-0 h-auto",
      },
      size: {
        sm: "h-6 px-2 text-[12px]",
        md: "h-7 px-2.5 text-[12.5px]",
        lg: "h-8 px-3.5 text-[13px]",
        "icon-sm": "h-6 w-6 px-0",
        icon: "h-7 w-7 px-0",
      },
    },
    defaultVariants: { variant: "secondary", size: "md" },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        className={cn(buttonVariants({ variant, size }), className)}
        ref={ref}
        {...props}
      />
    );
  },
);
Button.displayName = "Button";

export { buttonVariants };

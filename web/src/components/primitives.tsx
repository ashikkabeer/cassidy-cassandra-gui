import * as React from "react";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

type DotStatus = "green" | "red" | "amber" | "grey";

export function StatusDot({
  status = "green",
  label,
  mono,
  className,
}: {
  status?: DotStatus;
  label?: React.ReactNode;
  mono?: boolean;
  className?: string;
}) {
  return (
    <span className={cn("inline-flex items-center gap-1.5", className)}>
      <span className={cn("dot", `dot-${status}`)} />
      {label != null && (
        <span className={cn(mono && "mono", "text-[12px]")}>{label}</span>
      )}
    </span>
  );
}

export function Kbd({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <span className={cn("kbd", className)}>{children}</span>;
}

export function Spinner({
  size = 14,
  className,
}: {
  size?: number;
  className?: string;
}) {
  return (
    <Loader2
      size={size}
      strokeWidth={2}
      className={cn("animate-spin", className)}
    />
  );
}

export function Skeleton({
  w = "100%",
  h = 12,
  className,
  style,
}: {
  w?: number | string;
  h?: number | string;
  className?: string;
  style?: React.CSSProperties;
}) {
  return (
    <span
      className={cn("skel inline-block", className)}
      style={{ width: w, height: h, ...style }}
    />
  );
}

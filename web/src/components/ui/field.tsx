import * as React from "react";
import { Label } from "./label";

interface FieldProps {
  label?: React.ReactNode;
  hint?: React.ReactNode;
  help?: React.ReactNode;
  error?: React.ReactNode;
  children: React.ReactNode;
}

export function Field({ label, hint, help, error, children }: FieldProps) {
  return (
    <div>
      {label && <Label hint={hint}>{label}</Label>}
      {children}
      {help && !error && (
        <div className="text-[11px] text-muted-foreground mt-1">{help}</div>
      )}
      {error && (
        <div className="text-[11px] text-[hsl(0_80%_70%)] mt-1">{error}</div>
      )}
    </div>
  );
}

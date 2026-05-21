import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X, AlertTriangle } from "lucide-react";
import { cn } from "@/lib/utils";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogPortal = DialogPrimitive.Portal;
export const DialogClose = DialogPrimitive.Close;

export const DialogOverlay = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Overlay
    ref={ref}
    className={cn(
      "fixed inset-0 z-50 bg-[hsl(240_10%_2%/0.7)] backdrop-blur-[2px] data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
      className,
    )}
    {...props}
  />
));
DialogOverlay.displayName = DialogPrimitive.Overlay.displayName;

export interface DialogContentProps
  extends Omit<React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content>, "title"> {
  danger?: boolean;
  width?: number | string;
  subtitle?: React.ReactNode;
  title?: React.ReactNode;
  footer?: React.ReactNode;
  hideClose?: boolean;
}

export const DialogContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  DialogContentProps
>(
  (
    {
      className,
      children,
      danger,
      width = 560,
      subtitle,
      title,
      footer,
      hideClose,
      ...props
    },
    ref,
  ) => (
    <DialogPortal>
      <DialogOverlay />
      <DialogPrimitive.Content
        ref={ref}
        style={{ width }}
        className={cn(
          "fixed left-[50%] top-[50%] z-50 max-h-[90vh] translate-x-[-50%] translate-y-[-50%] flex flex-col rounded-[var(--radius)] border bg-popover shadow-[0_24px_64px_rgba(0,0,0,.6)] data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95",
          className,
        )}
        {...props}
      >
        {(title || subtitle) && (
          <div className="flex items-start gap-2.5 border-b px-3.5 py-3">
            {danger && (
              <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-[hsl(var(--destructive)/0.15)] text-[hsl(0_80%_75%)]">
                <AlertTriangle size={14} strokeWidth={1.8} />
              </div>
            )}
            <div className="flex-1">
              {title && <div className="text-[14px] font-semibold">{title}</div>}
              {subtitle && (
                <div className="mt-0.5 text-[12px] text-muted-foreground">
                  {subtitle}
                </div>
              )}
            </div>
            {!hideClose && (
              <DialogPrimitive.Close className="rounded-sm opacity-70 transition-opacity hover:opacity-100 focus:outline-none">
                <X size={14} />
                <span className="sr-only">Close</span>
              </DialogPrimitive.Close>
            )}
          </div>
        )}
        <div className="flex-1 overflow-auto p-3.5">{children}</div>
        {footer && (
          <div className="flex items-center gap-2 border-t px-3.5 py-2.5">
            {footer}
          </div>
        )}
      </DialogPrimitive.Content>
    </DialogPortal>
  ),
);
DialogContent.displayName = DialogPrimitive.Content.displayName;

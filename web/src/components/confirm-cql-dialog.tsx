import * as React from "react";
import { AlertTriangle, Check, Lock } from "lucide-react";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Panel } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Highlight } from "@/components/cql-highlight";
import { Spinner } from "@/components/primitives";

export interface ConfirmCqlDialogProps {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  cql: string;
  changeCount: number;
  deleteCount: number;
  committing: boolean;
  onConfirm: () => void;
}

// Confirm-CQL dialog — danger variant. Shows the server-generated BATCH that a
// commit will run, a tombstone warning when deletes are staged, and gates the
// destructive Run button behind an "I understand" checkbox.
export function ConfirmCqlDialog({
  open,
  onOpenChange,
  cql,
  changeCount,
  deleteCount,
  committing,
  onConfirm,
}: ConfirmCqlDialogProps) {
  const [acknowledged, setAcknowledged] = React.useState(false);

  React.useEffect(() => {
    if (!open) setAcknowledged(false);
  }, [open]);

  const needsAck = deleteCount > 0;
  const canRun = !committing && (!needsAck || acknowledged);

  const footer = (
    <>
      <div className="flex flex-1 items-center gap-2">
        <span className="text-[11px] text-muted-foreground">
          <Lock size={10} strokeWidth={1.8} className="-mb-0.5 mr-1 inline" />
          atomic LOGGED BATCH
        </span>
      </div>
      <Button variant="ghost" size="md" onClick={() => onOpenChange(false)}>
        Cancel
      </Button>
      <Button variant="destructive" size="md" disabled={!canRun} onClick={onConfirm}>
        {committing ? <Spinner size={11} /> : <Check size={11} strokeWidth={2.4} />}
        Run CQL
      </Button>
    </>
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        danger
        width={560}
        title={`Apply ${changeCount} pending change${changeCount === 1 ? "" : "s"}?`}
        subtitle="Cassandra will run the following CQL atomically. Inspect carefully."
        footer={footer}
      >
        <Panel className="mono max-h-[280px] overflow-auto px-2.5 py-2 text-[11px] leading-[1.65]">
          {cql ? <Highlight src={cql} /> : <span className="text-muted-foreground">…</span>}
        </Panel>

        {deleteCount > 0 && (
          <div className="mt-2.5 flex items-center gap-2 rounded-[var(--radius)] border border-[hsl(var(--warning)/0.25)] bg-[hsl(var(--warning)/0.08)] px-2.5 py-2">
            <AlertTriangle
              size={14}
              strokeWidth={1.8}
              className="shrink-0 text-[hsl(var(--warning))]"
            />
            <div className="text-[11.5px]">
              <strong className="font-semibold">
                {deleteCount} row{deleteCount === 1 ? "" : "s"} will be deleted permanently.
              </strong>{" "}
              <span className="text-muted-foreground">
                Cassandra deletes are tombstones — they can&apos;t be undone from the UI.
              </span>
            </div>
          </div>
        )}

        {needsAck && (
          <label className="mt-2.5 inline-flex cursor-pointer items-center gap-2 text-[11.5px]">
            <Checkbox
              checked={acknowledged}
              onCheckedChange={(v) => setAcknowledged(v === true)}
            />
            I understand — this permanently deletes data.
          </label>
        )}
      </DialogContent>
    </Dialog>
  );
}

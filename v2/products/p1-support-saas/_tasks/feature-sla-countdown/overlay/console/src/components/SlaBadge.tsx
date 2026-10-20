// A small status pill showing how a ticket stands against its SLA.

export function SlaBadge({ dueAt, now = new Date() }: { dueAt: string; now?: Date }) {
  // TODO(candidate): render the SLA state as a labelled pill. See TASK.md.
  void dueAt;
  void now;
  return (
    <span className="sla-badge" role="status">
      SLA
    </span>
  );
}

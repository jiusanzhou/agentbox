import { Badge } from "@/components/ui/badge";

const statusConfig = {
  pending: { label: "Pending", className: "bg-yellow-500/15 text-yellow-400 hover:bg-yellow-500/25 border-yellow-500/30" },
  running: { label: "Running", className: "bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 border-blue-500/30" },
  completed: { label: "Completed", className: "bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25 border-emerald-500/30" },
  failed: { label: "Failed", className: "bg-red-500/15 text-red-400 hover:bg-red-500/25 border-red-500/30" },
};

export function StatusBadge({ status }: { status: string }) {
  const config = statusConfig[status as keyof typeof statusConfig] || statusConfig.pending;
  return (
    <Badge variant="outline" className={config.className}>
      {config.label}
    </Badge>
  );
}

export function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

export function formatDuration(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 1000) return `${ms}ms`;
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rs = s % 60;
  return `${m}m ${rs}s`;
}

import { Skeleton } from "./Skeleton";
import { SurfaceCard } from "../SurfaceCard/SurfaceCard";

export interface SettingsGroupSkeletonProps {
  rows?: number;
}

export function SettingsGroupSkeleton({ rows = 3 }: SettingsGroupSkeletonProps) {
  return (
    <SurfaceCard className="p-0 overflow-hidden">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center justify-between p-4 border-b border-white/10 last:border-0">
          <div className="flex flex-col gap-2 w-1/2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
          <Skeleton className="h-8 w-24" />
        </div>
      ))}
    </SurfaceCard>
  );
}

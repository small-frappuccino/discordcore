import { Skeleton } from "./Skeleton";
import { SettingsGroup } from "../tahoe/SettingsGroup";

export interface SettingsGroupSkeletonProps {
  rows?: number;
}

export function SettingsGroupSkeleton({ rows = 3 }: SettingsGroupSkeletonProps) {
  return (
    <SettingsGroup>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="tahoe-settings-row">
          <div className="flex flex-col gap-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
          <div className="flex justify-end">
             <Skeleton className="h-8 w-24" />
          </div>
        </div>
      ))}
    </SettingsGroup>
  );
}

import { EmptyState } from "../../components/ui";
import { PartnerBoardWorkspaceState } from "./PartnerBoardWorkspaceState";
import { usePartnerBoard } from "./PartnerBoardContext";

export function PartnerBoardActivityPage() {
  const { workspaceState } = usePartnerBoard();

  if (workspaceState !== "ready") {
    return <PartnerBoardWorkspaceState />;
  }

  return (
    <EmptyState
      title="Activity is deferred"
      description="Phase 1 does not fake Partner Board history. Activity arrives after the backend exposes real event and audit data."
    />
  );
}

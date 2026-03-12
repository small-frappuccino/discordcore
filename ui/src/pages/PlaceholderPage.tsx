import { EmptyState, PageHeader, StatusBadge } from "../components/ui";

export function PlaceholderPage({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Planned area"
        title={title}
        description={description}
        status={<StatusBadge tone="neutral">Planned</StatusBadge>}
      />

      <div className="content-grid content-grid-single">
        <EmptyState
          title={`${title} is planned`}
          description="This section intentionally stays empty in phase 1 so the dashboard can establish a clean shell and page architecture first."
        />
      </div>
    </section>
  );
}

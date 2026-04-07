import { UnsavedChangesBar } from "../../components/ui";
import { PartnerBoardWorkspaceState } from "./PartnerBoardWorkspaceState";
import { usePartnerBoard } from "./PartnerBoardContext";

export function PartnerBoardLayoutPage() {
  const {
    layoutFieldCount,
    layoutDirty,
    layoutForm,
    loading,
    resetLayoutForm,
    saveLayout,
    setLayoutFormField,
    workspaceState,
  } = usePartnerBoard();

  if (workspaceState !== "ready") {
    return <PartnerBoardWorkspaceState />;
  }

  return (
    <section className="workspace-view">
      <div className="workspace-view-header">
        <div className="card-copy">
          <p className="section-label">Layout</p>
          <h2>Board text</h2>
          <p className="section-description">
            Focus the default workspace on the core copy fields people actually manage.
          </p>
        </div>
        <div className="workspace-view-meta">
          <span className="meta-pill subtle-pill">{layoutFieldCount}/5 fields filled</span>
        </div>
      </div>

      <UnsavedChangesBar
        hasUnsavedChanges={layoutDirty}
        saveLabel={loading ? "Saving..." : "Save changes"}
        saving={loading}
        onReset={resetLayoutForm}
        onSave={saveLayout}
      />

      <div className="workspace-form-grid">
        <label className="field-stack">
          <span className="field-label">Board title</span>
          <input
            value={layoutForm.title}
            onChange={(event) => setLayoutFormField("title", event.target.value)}
            placeholder="Partner Board"
          />
        </label>

        <label className="field-stack">
          <span className="field-label">Group heading</span>
          <input
            value={layoutForm.sectionHeaderTemplate}
            onChange={(event) =>
              setLayoutFormField("sectionHeaderTemplate", event.target.value)
            }
            placeholder="Section header"
          />
        </label>

        <label className="field-stack">
          <span className="field-label">Partner row text</span>
          <input
            value={layoutForm.lineTemplate}
            onChange={(event) =>
              setLayoutFormField("lineTemplate", event.target.value)
            }
            placeholder="Partner row"
          />
        </label>

        <label className="field-stack">
          <span className="field-label">Empty board message</span>
          <input
            value={layoutForm.emptyStateText}
            onChange={(event) =>
              setLayoutFormField("emptyStateText", event.target.value)
            }
            placeholder="No partners yet"
          />
        </label>
      </div>

      <label className="field-stack">
        <span className="field-label">Intro text</span>
        <textarea
          rows={6}
          value={layoutForm.intro}
          onChange={(event) => setLayoutFormField("intro", event.target.value)}
          placeholder="Optional introduction shown above the partner list"
        />
      </label>

      <details className="details-panel">
        <summary>Advanced formatting</summary>
        <div className="details-content">
          <p>
            Continuation text, footer options, sorting flags, and color controls stay
            out of the default workspace in phase 1. The UI keeps those backend fields
            intact, but does not promote them in the main editing flow.
          </p>
        </div>
      </details>
    </section>
  );
}

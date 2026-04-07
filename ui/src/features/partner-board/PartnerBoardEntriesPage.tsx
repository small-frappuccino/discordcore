import { UnsavedChangesBar } from "../../components/ui";
import { usePartnerBoard } from "./PartnerBoardContext";
import { PartnerBoardWorkspaceState } from "./PartnerBoardWorkspaceState";

export function PartnerBoardEntriesPage() {
  const {
    confirmDeleteEntry,
    drawerMode,
    entryDirty,
    entryForm,
    filteredPartners,
    isDrawerOpen,
    loading,
    openCreateEntryDrawer,
    openEditEntryDrawer,
    partners,
    pendingDeleteName,
    resetEntryForm,
    saveEntry,
    searchQuery,
    setEntryFormField,
    setSearchQuery,
    toggleDeleteEntry,
    workspaceState,
    closeEntryDrawer,
  } = usePartnerBoard();

  if (workspaceState !== "ready") {
    return <PartnerBoardWorkspaceState />;
  }

  return (
    <>
      <section className="workspace-view">
        <div className="workspace-view-header">
          <div className="card-copy">
            <p className="section-label">Partner entries</p>
            <h2>Manage entries</h2>
            <p className="section-description">
              Add, edit, and remove partner listings from one list instead of
              juggling separate CRUD forms.
            </p>
          </div>
          <div className="workspace-view-meta">
            <span className="meta-pill subtle-pill">
              {partners.length === 1 ? "1 partner" : `${partners.length} partners`}
            </span>
          </div>
        </div>

        <div className="workspace-toolbar">
          <label className="field-stack search-field workspace-search">
            <span className="field-label">Search</span>
            <input
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder="Search by group, name, or invite link"
            />
          </label>
          <div className="workspace-toolbar-actions">
            <button
              className="button-primary"
              type="button"
              onClick={openCreateEntryDrawer}
            >
              Add partner
            </button>
          </div>
        </div>

        {partners.length === 0 ? (
          <div className="table-empty-state">
            <div className="card-copy">
              <p className="section-label">Workspace</p>
              <h3>No partner entries yet</h3>
              <p className="section-description">
                Add the first partner to start building the board.
              </p>
            </div>
            <div className="workspace-state-actions">
              <button
                className="button-primary"
                type="button"
                onClick={openCreateEntryDrawer}
              >
                Add first partner
              </button>
            </div>
          </div>
        ) : filteredPartners.length === 0 ? (
          <div className="table-wrap">
            <div className="table-empty-state table-empty-state-compact">
              <div className="card-copy">
                <p className="section-label">Search results</p>
                <h3>No matching entries</h3>
                <p className="section-description">
                  Clear the search to see the full partner list again.
                </p>
              </div>
            </div>
          </div>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Group</th>
                  <th>Partner</th>
                  <th>Invite link</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filteredPartners.map((partner) => (
                  <tr key={`${partner.name}|${partner.link}`}>
                    <td>{partner.fandom?.trim() || "Other"}</td>
                    <td>{partner.name}</td>
                    <td>
                      <a
                        className="table-link"
                        href={partner.link}
                        rel="noreferrer"
                        target="_blank"
                      >
                        {partner.link}
                      </a>
                    </td>
                    <td>
                      {pendingDeleteName === partner.name ? (
                        <div className="inline-actions">
                          <button
                            className="button-danger"
                            type="button"
                            disabled={loading}
                            onClick={() => void confirmDeleteEntry(partner.name)}
                          >
                            Confirm
                          </button>
                          <button
                            className="button-ghost"
                            type="button"
                            onClick={() => toggleDeleteEntry(null)}
                          >
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <div className="inline-actions">
                          <button
                            className="button-secondary"
                            type="button"
                            disabled={loading}
                            onClick={() => openEditEntryDrawer(partner)}
                          >
                            Edit
                          </button>
                          <button
                            className="button-ghost"
                            type="button"
                            disabled={loading}
                            onClick={() => toggleDeleteEntry(partner.name)}
                          >
                            Remove
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {isDrawerOpen ? (
        <div className="drawer-backdrop" onClick={closeEntryDrawer} role="presentation">
          <aside
            aria-label={drawerMode === "edit" ? "Edit partner" : "Add partner"}
            className="drawer-panel"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="card-copy">
              <p className="section-label">
                {drawerMode === "edit" ? "Edit entry" : "New entry"}
              </p>
              <h2>{drawerMode === "edit" ? "Update partner" : "Add partner"}</h2>
              <p className="section-description">
                Entries save directly to the current board configuration in phase 1.
              </p>
            </div>

            <UnsavedChangesBar
              hasUnsavedChanges={entryDirty}
              saveLabel={loading ? "Saving..." : "Save changes"}
              saving={loading}
              onReset={resetEntryForm}
              onSave={saveEntry}
            />

            <label className="field-stack">
              <span className="field-label">Group</span>
              <input
                value={entryForm.fandom}
                onChange={(event) =>
                  setEntryFormField("fandom", event.target.value)
                }
                placeholder="Optional category label"
              />
            </label>

            <label className="field-stack">
              <span className="field-label">Partner name</span>
              <input
                value={entryForm.name}
                onChange={(event) => setEntryFormField("name", event.target.value)}
                placeholder="Partner name"
              />
            </label>

            <label className="field-stack">
              <span className="field-label">Invite link</span>
              <input
                value={entryForm.link}
                onChange={(event) => setEntryFormField("link", event.target.value)}
                placeholder="https://discord.gg/example"
              />
            </label>

          </aside>
        </div>
      ) : null}
    </>
  );
}

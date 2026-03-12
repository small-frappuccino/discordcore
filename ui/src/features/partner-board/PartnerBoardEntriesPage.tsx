import { EmptyState } from "../../components/ui";
import { usePartnerBoard } from "./PartnerBoardContext";
import { PartnerBoardWorkspaceState } from "./PartnerBoardWorkspaceState";

export function PartnerBoardEntriesPage() {
  const {
    confirmDeleteEntry,
    drawerMode,
    entryForm,
    filteredPartners,
    isDrawerOpen,
    loading,
    openCreateEntryDrawer,
    openEditEntryDrawer,
    partners,
    pendingDeleteName,
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
      <section className="surface-card">
        <div className="card-header">
          <div className="card-copy">
            <p className="section-label">Partner entries</p>
            <h2>Manage entries</h2>
            <p className="section-description">
              Add, edit, and remove partner listings from one list instead of
              juggling separate CRUD forms.
            </p>
          </div>

          <div className="card-actions">
            <label className="search-field">
              <span className="field-label">Search</span>
              <input
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="Search by group, name, or invite link"
              />
            </label>
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
          <EmptyState
            title="No partner entries yet"
            description="Add the first partner to start building the board."
            action={
              <button
                className="button-primary"
                type="button"
                onClick={openCreateEntryDrawer}
              >
                Add first partner
              </button>
            }
          />
        ) : filteredPartners.length === 0 ? (
          <EmptyState
            title="No matching entries"
            description="Clear the search to see the full partner list again."
          />
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

            <div className="drawer-actions">
              <button
                className="button-primary"
                type="button"
                disabled={loading}
                onClick={() => void saveEntry()}
              >
                {drawerMode === "edit" ? "Save changes" : "Add partner"}
              </button>
              <button
                className="button-secondary"
                type="button"
                onClick={closeEntryDrawer}
              >
                Cancel
              </button>
            </div>
          </aside>
        </div>
      ) : null}
    </>
  );
}

import { useEffect, useState } from "react";
import { useDashboardSession } from "../context/DashboardSessionContext";
import type { PartnerBoardTemplateConfig } from "../api/control";
import {
  PageHeader,
  SurfaceCard,
  SettingsGroup,
  SettingsRow,
  Button,
  Badge,
} from "../components";

export function PartnersPage() {
  const { client, selectedGuildID } = useDashboardSession();
  const [template, setTemplate] = useState<PartnerBoardTemplateConfig>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!selectedGuildID) return;
    setLoading(true);
    client
      .getPartnerBoard(selectedGuildID)
      .then((res) => {
        if (res.partner_board?.template) {
          setTemplate(res.partner_board.template);
        }
      })
      .catch((e) => console.error("Failed to load partner board template:", e))
      .finally(() => setLoading(false));
  }, [client, selectedGuildID]);

  const handleSave = async () => {
    if (!selectedGuildID) return;
    try {
      await client.setPartnerBoardTemplate(selectedGuildID, template);
      alert("Template saved successfully.");
    } catch (e) {
      console.error("Failed to save template:", e);
      alert("Failed to save template.");
    }
  };

  const updateField = (field: keyof PartnerBoardTemplateConfig, value: string) => {
    setTemplate((prev) => ({ ...prev, [field]: value }));
  };

  const inputStyle = {
    padding: "8px",
    borderRadius: "4px",
    border: "1px solid var(--border-subtle)",
    background: "var(--bg-base)",
    color: "var(--text-primary)",
    width: "250px",
  };

  if (!selectedGuildID) {
    return <div>Select a server to manage partners.</div>;
  }

  return (
    <div className="partners-page">
      <PageHeader
        title="Partner Board"
        description="Configure the template and layout for the automated partner board."
        badge={<Badge variant="neutral">Config</Badge>}
      />

      {loading ? (
        <div className="mt-8 text-muted">Loading settings...</div>
      ) : (
        <>
          <SurfaceCard className="mt-8">
            <SettingsGroup>
              <SettingsRow
                title="Board Title"
                description="The title of the partner board embed."
                control={
                  <input
                    type="text"
                    value={template.title || ""}
                    onChange={(e) => updateField("title", e.target.value)}
                    placeholder="Our Partners"
                    style={inputStyle}
                  />
                }
              />
              <SettingsRow
                title="Introduction Text"
                description="A message shown before the partner list."
                control={
                  <input
                    type="text"
                    value={template.intro || ""}
                    onChange={(e) => updateField("intro", e.target.value)}
                    placeholder="Check out these awesome servers!"
                    style={inputStyle}
                  />
                }
              />
              <SettingsRow
                title="Line Template"
                description="The format for each partner entry. E.g., `• {name}`"
                control={
                  <input
                    type="text"
                    value={template.line_template || ""}
                    onChange={(e) => updateField("line_template", e.target.value)}
                    placeholder="• {name}"
                    style={inputStyle}
                  />
                }
                isLast={true}
              />
            </SettingsGroup>
          </SurfaceCard>
          <div className="mt-4">
            <Button variant="primary" onClick={handleSave}>
              Save Template
            </Button>
          </div>
        </>
      )}
    </div>
  );
}

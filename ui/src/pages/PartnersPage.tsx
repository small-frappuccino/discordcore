import { PageHeader, SettingsGroup, SettingsRow, Button, Badge } from "../components/ui";
import { usePartnersPageLogic } from "./hooks/usePartnersPageLogic";
import type { Path } from "react-hook-form";
import type { PartnersFormData } from "./schemas/partners";

export function PartnersPage() {
  const {
    selectedGuildID,
    isLoading,
    isSaving,
    form,
    onSubmit,
  } = usePartnersPageLogic();

  if (!selectedGuildID) {
    return <div>Select a server to manage partner boards.</div>;
  }

  const renderInputRow = (name: Path<PartnersFormData>, title: string, placeholder: string = "") => (
    <SettingsRow
      title={title}
      description=""
      control={
        <input
          type="text"
          placeholder={placeholder}
          {...form.register(name)}
          style={{ width: "100%", padding: "8px", borderRadius: "4px", border: "1px solid var(--border-subtle)", background: "var(--bg-base)", color: "var(--text-primary)" }}
        />
      }
    />
  );

  const renderTextareaRow = (name: Path<PartnersFormData>, title: string) => (
    <SettingsRow
      title={title}
      description=""
      control={
        <textarea
          {...form.register(name)}
          style={{ width: "100%", minHeight: "60px", padding: "8px", borderRadius: "4px", border: "1px solid var(--border-subtle)", background: "var(--bg-base)", color: "var(--text-primary)" }}
        />
      }
    />
  );

  const renderCheckboxRow = (name: Path<PartnersFormData>, title: string) => (
    <SettingsRow
      title={title}
      description=""
      control={
        <input
          type="checkbox"
          {...form.register(name)}
        />
      }
    />
  );

  return (
    <form onSubmit={onSubmit}>
      <PageHeader
        title="Partner Board"
        description="Design how your automated partner board renders."
        badge={<Badge variant="success">Active</Badge>}
      />

      {isLoading ? (
        <div className="mt-8 text-muted">Loading partner template...</div>
      ) : (
        <div className="mt-8">
          <SettingsGroup>
            {renderInputRow("title", "Title", "Partner Board")}
            {renderInputRow("continuation_title", "Continuation Title", "Partner Board (Cont.)")}
            {renderTextareaRow("intro", "Intro")}
            {renderInputRow("section_header_template", "Section Header Template")}
            {renderInputRow("section_continuation_suffix", "Section Continuation Suffix")}
            {renderInputRow("section_continuation_template", "Section Continuation Template")}
            {renderInputRow("line_template", "Line Template")}
            {renderInputRow("empty_state_text", "Empty State Text")}
            {renderTextareaRow("footer_template", "Footer Template")}
            {renderInputRow("other_fandom_label", "Other Fandom Label")}
            <SettingsRow
              title="Embed Color (Decimal)"
              description="Color of the embed side border."
              control={
                <input
                  type="number"
                  {...form.register("color", { valueAsNumber: true })}
                  style={{ width: "150px", padding: "8px", borderRadius: "4px", border: "1px solid var(--border-subtle)", background: "var(--bg-base)", color: "var(--text-primary)" }}
                />
              }
            />
            {renderCheckboxRow("disable_fandom_sorting", "Disable Fandom Sorting")}
            {renderCheckboxRow("disable_partner_sorting", "Disable Partner Sorting")}
          </SettingsGroup>

          <div className="mt-4">
            <Button variant="primary" type="submit" disabled={isSaving}>
              {isSaving ? "Saving..." : "Save Template"}
            </Button>
          </div>
        </div>
      )}
    </form>
  );
}

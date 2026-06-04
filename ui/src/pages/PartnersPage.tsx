import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer } from "../components/ui";
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
    <SettingsRow>
      <SettingsRow.Info>
        <SettingsRow.Title>{title}</SettingsRow.Title>
      </SettingsRow.Info>
      <SettingsRow.Control>
        <input
          type="text"
          placeholder={placeholder}
          {...form.register(name)}
          className="form-input w-full"
        />
      </SettingsRow.Control>
    </SettingsRow>
  );

  const renderTextareaRow = (name: Path<PartnersFormData>, title: string) => (
    <SettingsRow>
      <SettingsRow.Info>
        <SettingsRow.Title>{title}</SettingsRow.Title>
      </SettingsRow.Info>
      <SettingsRow.Control>
        <textarea
          {...form.register(name)}
          className="form-input w-full min-h-16"
        />
      </SettingsRow.Control>
    </SettingsRow>
  );

  const renderCheckboxRow = (name: Path<PartnersFormData>, title: string) => (
    <SettingsRow>
      <SettingsRow.Info>
        <SettingsRow.Title>{title}</SettingsRow.Title>
      </SettingsRow.Info>
      <SettingsRow.Control>
        <input
          type="checkbox"
          className="form-checkbox w-4 h-4"
          {...form.register(name)}
        />
      </SettingsRow.Control>
    </SettingsRow>
  );

  return (
    <PageContainer>
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
            <SettingsRow>
              <SettingsRow.Info>
                <SettingsRow.Title>Embed Color (Decimal)</SettingsRow.Title>
                <SettingsRow.Description>Color of the embed side border.</SettingsRow.Description>
              </SettingsRow.Info>
              <SettingsRow.Control>
                <input
                  type="number"
                  {...form.register("color", { valueAsNumber: true })}
                  className="form-input w-40"
                />
              </SettingsRow.Control>
            </SettingsRow>
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
    </PageContainer>
  );
}

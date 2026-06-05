import { PageHeader, SettingsGroup, SettingsRow, Button, Badge, PageContainer, SettingsGroupSkeleton, ToggleSwitch } from "../components/ui";
import { Stack } from "../components/layout";
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
    <SettingsRow className="settings-row--multiline">
      <SettingsRow.Info>
        <SettingsRow.Title>{title}</SettingsRow.Title>
      </SettingsRow.Info>
      <SettingsRow.Control>
        <textarea
          {...form.register(name)}
          className="form-input w-full input-expansive"
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
        <ToggleSwitch {...form.register(name)} />
      </SettingsRow.Control>
    </SettingsRow>
  );

  return (
    <PageContainer>
      <form className="settings-form" onSubmit={onSubmit}>
        <Stack spacing="xl">
        <PageHeader>
          <PageHeader.TitleRow>
            <PageHeader.Title>Partner Board</PageHeader.Title>
            <Badge variant="success">Active</Badge>
          </PageHeader.TitleRow>
          <PageHeader.Description>Design how your automated partner board renders.</PageHeader.Description>
        </PageHeader>

        {isLoading ? (
          <SettingsGroupSkeleton rows={12} />
        ) : (
          <Stack spacing="lg">
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

            <div className="form-actions">
              <Button variant="primary" type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Template"}
              </Button>
            </div>
          </Stack>
        )}
        </Stack>
      </form>
    </PageContainer>
  );
}

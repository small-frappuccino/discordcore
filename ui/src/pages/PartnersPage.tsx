import { PageHeader, Badge, PageContainer, SettingsGroupSkeleton } from "../components/ui";
import {
  SettingsGroup,
  SettingsRow,
  ToggleSwitch,
  TextInput,
  TextArea,
  SaveActionBar
} from "../components/ui/tahoe";
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
    <SettingsRow
      title={title}
      control={<TextInput placeholder={placeholder} {...form.register(name)} className="w-full" />}
    />
  );

  const renderTextareaRow = (name: Path<PartnersFormData>, title: string) => (
    <SettingsRow
      isMultiline
      title={title}
      control={<TextArea {...form.register(name)} className="w-full input-expansive" />}
    />
  );

  const renderCheckboxRow = (name: Path<PartnersFormData>, title: string) => (
    <SettingsRow
      title={title}
      control={<ToggleSwitch {...form.register(name)} />}
    />
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
              <SettingsRow
                title="Embed Color (Decimal)"
                description="Color of the embed side border."
                control={
                  <TextInput
                    type="number"
                    {...form.register("color", { valueAsNumber: true })}
                    className="w-40"
                  />
                }
              />
              {renderCheckboxRow("disable_fandom_sorting", "Disable Fandom Sorting")}
              {renderCheckboxRow("disable_partner_sorting", "Disable Partner Sorting")}
            </SettingsGroup>
          </Stack>
        )}
        </Stack>
        <SaveActionBar
          isDirty={form.formState.isDirty}
          isSaving={isSaving}
          onSave={onSubmit}
          onReset={() => form.reset()}
        />
      </form>
    </PageContainer>
  );
}

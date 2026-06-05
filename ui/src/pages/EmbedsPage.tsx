import { PageHeader, Badge, PageContainer } from "../components/ui";
import { Stack } from "../components/layout";
import { useEmbedsPageLogic } from "./hooks/useEmbedsPageLogic";
import { EmbedSidebarList } from "./components/EmbedSidebarList";
import { EmbedEditorForm } from "./components/EmbedEditorForm";
import { EmbedLivePreview } from "./components/EmbedLivePreview";

export function EmbedsPage() {
  const {
    embeds,
    isLoading,
    selectedEmbedKey,
    selectEmbed,
    createNewEmbed,
    form,
    customFields,
    appendField,
    removeField,
    onSubmit,
    isSaving,
    isDeleting,
    deleteEmbed
  } = useEmbedsPageLogic();

  return (
    <PageContainer>
      <Stack spacing="lg">
        <PageHeader>
          <PageHeader.TitleRow>
            <PageHeader.Title>Custom Embeds</PageHeader.Title>
            <Badge variant="success">Active</Badge>
          </PageHeader.TitleRow>
          <PageHeader.Description>Design and manage custom embeds for your server.</PageHeader.Description>
        </PageHeader>

        <Stack direction="horizontal" spacing="xl" align="start" className="h-full">
          {/* Left Pane: List & Editor */}
          <Stack spacing="lg" className="flex-1">
            <EmbedSidebarList
              isLoading={isLoading}
              embeds={embeds}
              selectedEmbedKey={selectedEmbedKey}
              selectEmbed={selectEmbed}
              createNewEmbed={createNewEmbed}
            />

            <EmbedEditorForm
              form={form}
              customFields={customFields}
              appendField={appendField}
              removeField={removeField}
              onSubmit={onSubmit}
              isSaving={isSaving}
              isDeleting={isDeleting}
              deleteEmbed={deleteEmbed}
              selectedEmbedKey={selectedEmbedKey}
              activeEmbedDataKey={form.getValues("key")}
            />
          </Stack>

          {/* Right Pane: Live Preview */}
          <EmbedLivePreview control={form.control} />
        </Stack>
      </Stack>
    </PageContainer>
  );
}

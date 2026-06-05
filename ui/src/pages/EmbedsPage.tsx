import { PageHeader, Badge, PageContainer } from "../components/ui";
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
      <div className="flex flex-col gap-6">
        <PageHeader 
          title="Custom Embeds" 
          description="Design and manage custom embeds for your server."
          badge={<Badge variant="success">Active</Badge>}
        />

        <div className="flex gap-8 h-full items-start">
          {/* Left Pane: List & Editor */}
          <div className="flex-1 flex flex-col gap-6">
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
          </div>

          {/* Right Pane: Live Preview */}
          <EmbedLivePreview control={form.control} />
        </div>
      </div>
    </PageContainer>
  );
}

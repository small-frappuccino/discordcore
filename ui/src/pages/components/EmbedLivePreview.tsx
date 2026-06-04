import { useWatch, type Control } from "react-hook-form";
import { EmbedPreview } from "../../components/ui";
import type { EmbedsFormData } from "../schemas/embeds";

type EmbedLivePreviewProps = {
  control: Control<EmbedsFormData>;
};

export function EmbedLivePreview({ control }: EmbedLivePreviewProps) {
  const activeEmbedData = useWatch({ control });

  return (
    <div className="w-full max-w-lg shrink-0 sticky top-8">
      <h2 className="text-lg font-semibold text-white mb-4">Live Preview</h2>
      <div className="p-4 bg-[#36393f] rounded-lg border border-black/20 shadow-xl overflow-hidden">
        {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
        <EmbedPreview embed={activeEmbedData as any} />
      </div>
    </div>
  );
}

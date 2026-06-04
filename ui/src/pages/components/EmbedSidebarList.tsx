import { Button } from "../../components/ui";

type EmbedSidebarListProps = {
  isLoading: boolean;
  embeds: { key: string }[];
  selectedEmbedKey: string | null;
  selectEmbed: (emb: { key: string }) => void;
  createNewEmbed: () => void;
};

export function EmbedSidebarList({
  isLoading,
  embeds,
  selectedEmbedKey,
  selectEmbed,
  createNewEmbed,
}: EmbedSidebarListProps) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex justify-between items-center">
        <h2 className="text-lg font-semibold text-white">Saved Embeds</h2>
        <Button variant="primary" onClick={createNewEmbed} className="text-sm px-3 py-1">
          + New Embed
        </Button>
      </div>

      {isLoading ? (
        <p className="text-muted">Loading Embeds...</p>
      ) : embeds.length === 0 ? (
        <div className="text-muted text-sm italic py-4">No custom embeds found. Create one above!</div>
      ) : (
        <div className="flex flex-wrap gap-2">
          {embeds.map((emb) => (
            <button
              key={emb.key}
              type="button"
              onClick={() => selectEmbed(emb)}
              className={`px-4 py-2 rounded-md border text-sm font-medium transition-colors ${
                selectedEmbedKey === emb.key
                  ? "border-brand-500 bg-brand-500/10 text-white"
                  : "border-white/10 bg-white/5 text-muted hover:bg-white/10 hover:text-white"
              }`}
            >
              {emb.key}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

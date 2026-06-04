import type { CustomEmbedConfig } from "../../api/domains/embeds";

interface EmbedPreviewProps {
  embed: Partial<CustomEmbedConfig>;
}

export function EmbedPreview({ embed }: EmbedPreviewProps) {
  const colorHex = embed.color ? `#${embed.color.toString(16).padStart(6, '0')}` : '#202225';
  
  return (
    <div className="bg-[#36393f] text-[#dcddde] p-4 rounded-md font-sans w-full max-w-lg flex">
      {/* Left colored pill */}
      <div 
        className="w-1 rounded-l-md shrink-0" 
        style={{ backgroundColor: colorHex }} 
      />
      
      {/* Embed Content */}
      <div className="bg-[#2f3136] p-4 rounded-r-md flex-1 min-w-0 border border-black/10 shadow-sm flex flex-col gap-2">
        {/* Author */}
        {(embed.author_name || embed.author_icon_url) && (
          <div className="flex items-center gap-2 mb-1">
            {embed.author_icon_url && (
              <img src={embed.author_icon_url} alt="author icon" className="w-6 h-6 rounded-full object-cover" />
            )}
            {embed.author_name && <span className="font-semibold text-sm text-white">{embed.author_name}</span>}
          </div>
        )}

        {/* Title & Description & Thumbnail */}
        <div className="flex gap-4">
          <div className="flex-1 flex flex-col gap-2 min-w-0">
            {embed.title && (
              <div className="font-bold text-white text-base truncate">
                {embed.title}
              </div>
            )}
            {embed.description && (
              <div className="text-sm whitespace-pre-wrap break-words">
                {embed.description}
              </div>
            )}
          </div>
          {embed.thumbnail_url && (
            <div className="shrink-0">
              <img src={embed.thumbnail_url} alt="thumbnail" className="w-20 h-20 rounded object-cover" />
            </div>
          )}
        </div>

        {/* Fields */}
        {embed.fields && embed.fields.length > 0 && (
          <div className="grid grid-cols-12 gap-2 mt-2">
            {embed.fields.map((field, idx) => (
              <div key={idx} className={`${field.inline ? 'col-span-4' : 'col-span-12'} flex flex-col gap-1 min-w-0`}>
                <div className="text-xs font-semibold text-white truncate">{field.name || "\u200B"}</div>
                <div className="text-sm break-words">{field.value || "\u200B"}</div>
              </div>
            ))}
          </div>
        )}

        {/* Main Image */}
        {embed.image_url && (
          <div className="mt-2 rounded overflow-hidden max-w-md">
            <img src={embed.image_url} alt="embed image" className="w-full h-auto object-cover" />
          </div>
        )}

        {/* Footer */}
        {(embed.footer_text || embed.footer_icon_url) && (
          <div className="flex items-center gap-2 mt-2">
            {embed.footer_icon_url && (
              <img src={embed.footer_icon_url} alt="footer icon" className="w-5 h-5 rounded-full object-cover" />
            )}
            {embed.footer_text && <span className="text-xs">{embed.footer_text}</span>}
          </div>
        )}
      </div>
    </div>
  );
}

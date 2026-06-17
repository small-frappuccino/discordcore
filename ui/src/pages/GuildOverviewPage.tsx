import React, { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { PageContainer, PageHeader, SurfaceCard } from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { getBotProfiles, type BotProfile } from "../api/domains/guilds";

export const GuildOverviewPage: React.FC = () => {
  const { guildId } = useParams<{ guildId: string }>();
  const { client } = useDashboardSession();
  const [profiles, setProfiles] = useState<BotProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!guildId) return;
    setLoading(true);
    getBotProfiles(client, guildId)
      .then(data => setProfiles(data))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [guildId, client]);

  return (
    <PageContainer>
      <PageHeader>
        <PageHeader.TitleRow>
          <PageHeader.Title>Guild Overview</PageHeader.Title>
        </PageHeader.TitleRow>
        <PageHeader.Description>Select a bot profile to manage</PageHeader.Description>
      </PageHeader>

      {loading && <p>Loading profiles...</p>}
      {error && <p className="text-status-danger">{error}</p>}
      
      {!loading && !error && profiles.length === 0 && (
        <SurfaceCard>
          <div className="p-6 text-center text-text-secondary">
            No bot profiles configured for this guild.
          </div>
        </SurfaceCard>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {!loading && !error && profiles.map(profile => (
          <Link 
            key={profile.logical_key} 
            to={`/manage/${guildId}/bots/${encodeURIComponent(profile.logical_key)}/core`}
            className="block"
          >
            <SurfaceCard className="h-full hover:bg-surface-hover transition-colors cursor-pointer">
              <div className="p-4 flex items-center gap-4">
                {profile.avatar_url ? (
                  <img src={profile.avatar_url} alt={profile.username} className="w-12 h-12 rounded-full" />
                ) : (
                  <div className="w-12 h-12 rounded-full bg-surface-active flex items-center justify-center font-bold">
                    {profile.username.charAt(0).toUpperCase()}
                  </div>
                )}
                <div>
                  <h3 className="font-bold text-lg text-text-primary">{profile.username}</h3>
                  <p className="text-sm text-text-secondary">Key: {profile.logical_key}</p>
                </div>
              </div>
            </SurfaceCard>
          </Link>
        ))}
      </div>
    </PageContainer>
  );
};

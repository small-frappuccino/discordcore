import { memo } from "react";
import { useSettingsModal } from "../../context/SettingsModalContext";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { GeneralSettings } from "./GeneralSettings";
import { GuildDangerZone } from "./GuildDangerZone";
import { motion, AnimatePresence } from "framer-motion";

export const SettingsModal = memo(function SettingsModal() {
  const { isOpen, closeSettings, activeTab, setActiveTab } = useSettingsModal();
  const { manageableGuilds } = useDashboardSession();

  if (!isOpen) return null;

  return (
    <AnimatePresence>
      <div className="fixed inset-0 z-50 flex justify-center items-center">
        {/* Backdrop */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="absolute inset-0 bg-black/70 backdrop-blur-sm"
          onClick={closeSettings}
        />

        {/* Modal */}
        <motion.div
          initial={{ scale: 0.95, opacity: 0, y: 10 }}
          animate={{ scale: 1, opacity: 1, y: 0 }}
          exit={{ scale: 0.95, opacity: 0, y: 10 }}
          className="relative flex w-full max-w-5xl h-[85vh] bg-surface-base rounded-2xl shadow-2xl overflow-hidden border border-border-subtle"
        >
          {/* Sidebar */}
          <aside className="w-64 bg-surface-active flex-shrink-0 flex flex-col py-6 border-r border-border-subtle overflow-y-auto">
            <nav className="flex flex-col gap-1 px-3">
              <h3 className="px-3 text-xs font-bold text-text-muted uppercase tracking-wider mb-2">User Settings</h3>
              <SidebarItem 
                label="General" 
                active={activeTab === "general"} 
                onClick={() => setActiveTab("general")} 
              />
              <SidebarItem 
                label="Account" 
                active={activeTab === "account"} 
                onClick={() => setActiveTab("account")} 
              />

              <div className="my-4 border-t border-border-subtle mx-3" />

              <h3 className="px-3 text-xs font-bold text-text-muted uppercase tracking-wider mb-2">Projects</h3>
              {manageableGuilds.map(guild => (
                <SidebarItem 
                  key={guild.id}
                  label={guild.name} 
                  active={activeTab === `guild-${guild.id}`} 
                  onClick={() => setActiveTab(`guild-${guild.id}`)} 
                />
              ))}
            </nav>
          </aside>

          {/* Main Content Area */}
          <main className="flex-1 overflow-y-auto p-10 relative bg-bg-surface-base">
            <button 
              className="absolute top-6 right-6 w-8 h-8 flex items-center justify-center rounded-full bg-surface-active hover:bg-surface-hover text-text-secondary hover:text-text-primary transition-colors"
              onClick={closeSettings}
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M14 1.41L12.59 0L7 5.59L1.41 0L0 1.41L5.59 7L0 12.59L1.41 14L7 8.41L12.59 14L14 12.59L8.41 7L14 1.41Z" fill="currentColor"/>
              </svg>
            </button>

            <div className="max-w-2xl">
              {activeTab === "general" && <GeneralSettings />}
              {activeTab === "account" && (
                <div>
                  <h2 className="text-2xl font-bold mb-6">Account Settings</h2>
                  <p className="text-text-secondary">Manage your Discordcore account settings here.</p>
                </div>
              )}
              {activeTab.startsWith("guild-") && (
                <GuildDangerZone guildId={activeTab.replace("guild-", "")} />
              )}
            </div>
          </main>
        </motion.div>
      </div>
    </AnimatePresence>
  );
});

function SidebarItem({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      className={`text-left px-3 py-2 rounded-md text-sm transition-colors font-medium ${
        active 
          ? "bg-brand-primary/10 text-brand-primary" 
          : "text-text-secondary hover:bg-surface-hover hover:text-text-primary"
      }`}
      onClick={onClick}
    >
      {label}
    </button>
  );
}

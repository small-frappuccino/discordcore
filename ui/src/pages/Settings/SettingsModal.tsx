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
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-8">
        {/* Subtle Darkened Backdrop */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="absolute inset-0 bg-black/40"
          onClick={closeSettings}
        />

        {/* Floating Modal Component */}
        <motion.div
          initial={{ scale: 0.98, opacity: 0, y: 10 }}
          animate={{ scale: 1, opacity: 1, y: 0 }}
          exit={{ scale: 0.98, opacity: 0, y: 10 }}
          className="relative flex w-full max-w-[1100px] h-[85vh] min-h-[500px] bg-base rounded-2xl shadow-2xl overflow-hidden border border-border-subtle"
        >
          {/* Sidebar */}
          <aside className="w-64 bg-surface flex flex-col shrink-0 border-r border-border-subtle pt-4 pb-6">
            <nav className="shell-nav px-2">
              <div className="shell-nav-section-title px-3">User Settings</div>
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
              <div className="shell-nav-section-title px-3 mt-2">Projects</div>
              {manageableGuilds.map((guild) => (
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
          <main className="flex-1 overflow-y-auto relative bg-base pt-10 px-10 pb-8">
            <div className="max-w-2xl mx-auto w-full">
              {activeTab === "general" && <GeneralSettings />}
              {activeTab === "account" && (
                <div>
                  <h2 className="text-2xl font-bold mb-6">Account Settings</h2>
                  <p className="text-secondary">Manage your Discordcore account settings here.</p>
                </div>
              )}
              {activeTab.startsWith("guild-") && (
                <GuildDangerZone guildId={activeTab.replace("guild-", "")} />
              )}
            </div>

            {/* Esc Button Area */}
            <div className="absolute top-6 right-6 flex items-center justify-center z-50">
              <button 
                className="group w-10 h-10 flex items-center justify-center rounded-xl bg-transparent hover:bg-surface-active transition-colors duration-200 cursor-pointer"
                onClick={closeSettings}
              >
                <svg className="text-secondary group-hover:text-white transition-colors duration-200" width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M14 1.41L12.59 0L7 5.59L1.41 0L0 1.41L5.59 7L0 12.59L1.41 14L7 8.41L12.59 14L14 12.59L8.41 7L14 1.41Z" fill="currentColor"/>
                </svg>
              </button>
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
      className={`shell-nav-link w-full text-left justify-start ${active ? "is-active" : ""}`}
      onClick={onClick}
    >
      <span>{label}</span>
    </button>
  );
}

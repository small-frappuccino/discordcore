import { useState, useRef, useEffect, memo } from "react";
import { useDashboardSession } from "../../context/DashboardSessionContext";

export const AccountSelector = memo(function AccountSelector() {
  const { session, sessionAvatarURL, logout } = useDashboardSession();

  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false);
  const accountMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (accountMenuRef.current && !accountMenuRef.current.contains(event.target as Node)) {
        setIsAccountMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const accountTitle = session?.user?.username || "Unknown User";
  const avatarUrl = sessionAvatarURL || "https://cdn.discordapp.com/embed/avatars/0.png";

  return (
    <div className="relative" ref={accountMenuRef}>
      <button 
        className="shell-trigger-btn hover:bg-[var(--bg-surface-hover)] active:scale-[0.98] transition-all"
        onClick={() => setIsAccountMenuOpen(!isAccountMenuOpen)}
      >
        <div className="shell-trigger-avatar">
          <img src={avatarUrl} alt="Avatar" />
        </div>
        <div className="shell-trigger-info">
          <span className="shell-trigger-title">{accountTitle}</span>
        </div>
        <span className="shell-trigger-chevron">v</span>
      </button>

      <div 
        className={`shell-dropdown transition-all duration-200 ease-out origin-top-right ${
          isAccountMenuOpen ? 'opacity-100 scale-100 pointer-events-auto' : 'opacity-0 scale-95 pointer-events-none'
        }`}
      >
        <div className="px-3 py-2 border-b border-subtle mb-1">
          <div className="text-sm font-semibold">{accountTitle}</div>
          <div className="text-xs text-muted">{session?.user?.id}</div>
        </div>
        <button
          className="shell-dropdown-item danger"
          onClick={() => {
            setIsAccountMenuOpen(false);
            logout();
          }}
        >
          Log Out
        </button>
      </div>
    </div>
  );
});

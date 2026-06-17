import { useState, useRef, useEffect, memo } from "react";
import { useDashboardSession } from "../../context/DashboardSessionContext";

export const AccountSelector = memo(function AccountSelector() {
  const { session, sessionAvatarURL, logout, authState } = useDashboardSession();

  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false);
  const accountMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (accountMenuRef.current && !accountMenuRef.current.contains(event.target as Node)) {
        setIsAccountMenuOpen(false);
      }
    }
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setIsAccountMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  if (authState === "checking") {
    return (
      <div className="relative">
        <button className="shell-trigger-btn opacity-50 cursor-wait">
          <div className="shell-trigger-avatar animate-pulse bg-border-subtle" />
          <div className="shell-trigger-info animate-pulse flex flex-col gap-2 justify-center">
            <div className="h-3 bg-border-subtle rounded w-24"></div>
            <div className="h-2.5 bg-border-subtle rounded w-16"></div>
          </div>
        </button>
      </div>
    );
  }

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
          <span className="shell-trigger-subtitle">Manage Account</span>
        </div>
        <span className="shell-trigger-chevron">v</span>
      </button>

      <div 
        className={`shell-dropdown transition-all duration-200 ease-out origin-top-right ${
          isAccountMenuOpen ? 'opacity-100 scale-100 pointer-events-auto' : 'opacity-0 scale-95 pointer-events-none'
        }`}
      >
        <div className="px-3 py-2 mb-1">
          <div className="text-sm font-semibold">{accountTitle}</div>
          <div className="text-xs text-muted truncate">{session?.user?.id}</div>
        </div>
        <div className="shell-dropdown-divider"></div>
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

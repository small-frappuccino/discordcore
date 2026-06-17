/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, type ReactNode } from "react";

interface SettingsModalContextValue {
  isOpen: boolean;
  openSettings: (tab?: string) => void;
  closeSettings: () => void;
  activeTab: string;
  setActiveTab: (tab: string) => void;
}

const SettingsModalContext = createContext<SettingsModalContextValue | null>(null);

export function SettingsModalProvider({ children }: { children: ReactNode }) {
  const [isOpen, setIsOpen] = useState(false);
  const [activeTab, setActiveTab] = useState("general");

  const openSettings = (tab?: string) => {
    if (tab) setActiveTab(tab);
    setIsOpen(true);
  };

  const closeSettings = () => {
    setIsOpen(false);
  };

  return (
    <SettingsModalContext.Provider
      value={{
        isOpen,
        openSettings,
        closeSettings,
        activeTab,
        setActiveTab,
      }}
    >
      {children}
    </SettingsModalContext.Provider>
  );
}

export function useSettingsModal() {
  const context = useContext(SettingsModalContext);
  if (!context) {
    throw new Error("useSettingsModal must be used within SettingsModalProvider");
  }
  return context;
}

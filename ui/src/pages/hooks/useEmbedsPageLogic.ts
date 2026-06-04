import { useEffect, useState } from "react";

export interface EmbedsConfig {
  webhook_url: string;
  enabled: boolean;
}

export function useEmbedsPageLogic() {
  const [config, setConfig] = useState<EmbedsConfig | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    // Mocking the fetch call
    const timer = setTimeout(() => {
      setConfig({
        webhook_url: "https://discord.com/api/webhooks/...",
        enabled: true,
      });
      setIsLoading(false);
    }, 500);
    return () => clearTimeout(timer);
  }, []);

  const handleSave = () => {
    if (!config) return;
    setIsSaving(true);
    // Mocking the save call
    setTimeout(() => {
      setIsSaving(false);
    }, 500);
  };

  return {
    config,
    setConfig,
    isLoading,
    isSaving,
    handleSave,
  };
}

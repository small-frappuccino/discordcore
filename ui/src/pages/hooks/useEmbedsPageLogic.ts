import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import toast from "react-hot-toast";
import { EmbedsSchema, type EmbedsFormData } from "../schemas/embeds";

export function useEmbedsPageLogic() {
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);

  const form = useForm<EmbedsFormData>({
    resolver: zodResolver(EmbedsSchema),
    defaultValues: {
      webhook_url: "",
      enabled: false,
    }
  });

  useEffect(() => {
    // Mocking the fetch call
    const timer = setTimeout(() => {
      form.reset({
        webhook_url: "https://discord.com/api/webhooks/...",
        enabled: true,
      });
      setIsLoading(false);
    }, 500);
    return () => clearTimeout(timer);
  }, [form]);

  const onSubmit = form.handleSubmit(() => {
    setIsSaving(true);
    // Mocking the save call
    setTimeout(() => {
      setIsSaving(false);
      toast.success("Embeds Settings saved!");
    }, 500);
  });

  return {
    form,
    onSubmit,
    isLoading,
    isSaving,
  };
}

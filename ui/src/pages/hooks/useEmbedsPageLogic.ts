import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm, useFieldArray } from "react-hook-form";
import { useParams } from "react-router-dom";
import { zodResolver } from "@hookform/resolvers/zod";
import toast from "react-hot-toast";

import { useDashboardSession } from "../../context/DashboardSessionContext";
import { EmbedsSchema, type EmbedsFormData } from "../schemas/embeds";
import { getCustomEmbeds, putCustomEmbed, deleteCustomEmbed, type CustomEmbedConfig } from "../../api/domains/embeds";

export function useEmbedsPageLogic() {
  const { client } = useDashboardSession();
  const { guildId = "" } = useParams<{ guildId: string }>();
  const queryClient = useQueryClient();

  const [selectedEmbedKey, setSelectedEmbedKey] = useState<string | null>(null);

  const { data: embeds = [], isLoading } = useQuery({
    queryKey: ["embeds", guildId],
    queryFn: () => getCustomEmbeds(client, guildId),
    enabled: !!guildId && !!client,
  });

  const form = useForm<EmbedsFormData>({
    resolver: zodResolver(EmbedsSchema),
    defaultValues: {
      key: "",
      title: "",
      description: "",
      color: 0x5865F2,
      author_name: "",
      author_icon_url: "",
      footer_text: "",
      footer_icon_url: "",
      image_url: "",
      thumbnail_url: "",
      fields: [],
    }
  });

  const { fields: customFields, append, remove } = useFieldArray({
    control: form.control,
    name: "fields",
  });

  const selectEmbed = (embed: CustomEmbedConfig) => {
    setSelectedEmbedKey(embed.key);
    form.reset({
      ...embed,
      fields: embed.fields || [],
      postings: embed.postings || [],
    });
  };

  const createNewEmbed = () => {
    setSelectedEmbedKey(null);
    form.reset({
      key: `embed_${Date.now()}`,
      title: "New Embed",
      description: "",
      color: 0x5865F2,
      fields: [],
      postings: [],
    });
  };

  const saveMutation = useMutation({
    mutationFn: (data: EmbedsFormData) => putCustomEmbed(client, guildId, data.key, data as CustomEmbedConfig),
    onSuccess: (savedEmbed) => {
      queryClient.invalidateQueries({ queryKey: ["embeds", guildId] });
      toast.success("Embed saved successfully!");
      setSelectedEmbedKey(savedEmbed.key);
    },
    onError: () => toast.error("Failed to save embed.")
  });

  const deleteMutation = useMutation({
    mutationFn: (key: string) => deleteCustomEmbed(client, guildId, key),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["embeds", guildId] });
      toast.success("Embed deleted!");
      setSelectedEmbedKey(null);
      form.reset({
        key: "",
        title: "",
        description: "",
        color: 0x5865F2,
        author_name: "",
        author_icon_url: "",
        footer_text: "",
        footer_icon_url: "",
        image_url: "",
        thumbnail_url: "",
        fields: [],
      });
    },
    onError: () => toast.error("Failed to delete embed.")
  });

  const onSubmit = form.handleSubmit((data) => {
    saveMutation.mutate(data);
  });

  return {
    embeds,
    isLoading,
    selectedEmbedKey,
    selectEmbed,
    createNewEmbed,
    form,
    customFields,
    appendField: () => append({ name: "New Field", value: "Field Value", inline: false }),
    removeField: remove,
    onSubmit,
    isSaving: saveMutation.isPending,
    isDeleting: deleteMutation.isPending,
    deleteEmbed: () => {
      if (selectedEmbedKey) deleteMutation.mutate(selectedEmbedKey);
    }
  };
}

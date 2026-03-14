/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import type { PartnerBoardConfig, PartnerEntryConfig } from "../../api/control";
import { formatError } from "../../app/utils";
import type { Notice } from "../../app/types";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import {
  buildDeliveryPayload,
  buildLayoutPayload,
  countFilledLayoutFields,
  formsFromBoard,
  getPartnerBoardShellStatus,
  initialDeliveryForm,
  initialEntryForm,
  initialLayoutForm,
  isDeliveryConfigured,
  isLayoutConfigured,
  summarizePostingDestination,
  validateDeliveryForm,
  type DeliveryFormState,
  type EntryFormState,
  type LayoutFormState,
} from "./model";

type EntryDrawerMode = "create" | "edit";
type WorkspaceState =
  | "auth_required"
  | "checking"
  | "loading"
  | "ready"
  | "server_required"
  | "unavailable";

interface PartnerBoardContextValue {
  board: PartnerBoardConfig | null;
  busyLabel: string;
  deliveryConfigured: boolean;
  deliveryForm: DeliveryFormState;
  drawerMode: EntryDrawerMode;
  entryForm: EntryFormState;
  filteredPartners: PartnerEntryConfig[];
  hasLoadedAttempt: boolean;
  isDrawerOpen: boolean;
  lastLoadedAt: number | null;
  lastSyncedAt: number | null;
  layoutConfigured: boolean;
  layoutFieldCount: number;
  layoutForm: LayoutFormState;
  loading: boolean;
  notice: Notice | null;
  partners: PartnerEntryConfig[];
  pendingDeleteName: string | null;
  searchQuery: string;
  shellStatus: ReturnType<typeof getPartnerBoardShellStatus>;
  workspaceState: WorkspaceState;
  clearNotice: () => void;
  closeEntryDrawer: () => void;
  confirmDeleteEntry: (partnerName: string) => Promise<void>;
  openCreateEntryDrawer: () => void;
  openEditEntryDrawer: (partner: PartnerEntryConfig) => void;
  refreshBoard: () => Promise<void>;
  saveDelivery: () => Promise<void>;
  saveEntry: () => Promise<void>;
  saveLayout: () => Promise<void>;
  setDeliveryFormField: (
    field: keyof DeliveryFormState,
    value: string,
  ) => void;
  setEntryFormField: (field: keyof EntryFormState, value: string) => void;
  setLayoutFormField: (field: keyof LayoutFormState, value: string) => void;
  setSearchQuery: (value: string) => void;
  summarizePostingDestination: string;
  syncBoard: () => Promise<void>;
  toggleDeleteEntry: (partnerName: string | null) => void;
}

const PartnerBoardContext =
  createContext<PartnerBoardContextValue | null>(null);

export function PartnerBoardProvider({ children }: { children: ReactNode }) {
  const { authState, canManageGuild, client, selectedGuildID } =
    useDashboardSession();
  const [board, setBoard] = useState<PartnerBoardConfig | null>(null);
  const [deliveryForm, setDeliveryForm] = useState(initialDeliveryForm);
  const [layoutForm, setLayoutForm] = useState(initialLayoutForm);
  const [entryForm, setEntryForm] = useState(initialEntryForm);
  const [searchQuery, setSearchQuery] = useState("");
  const [pendingDeleteName, setPendingDeleteName] = useState<string | null>(null);
  const [editingPartnerName, setEditingPartnerName] = useState("");
  const [drawerMode, setDrawerMode] = useState<EntryDrawerMode>("create");
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [loading, setLoading] = useState(false);
  const [busyLabel, setBusyLabel] = useState("");
  const [lastLoadedAt, setLastLoadedAt] = useState<number | null>(null);
  const [lastSyncedAt, setLastSyncedAt] = useState<number | null>(null);
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(false);

  const partners = board?.partners ?? [];
  const deliveryDraft = buildDeliveryPayload(deliveryForm);
  const layoutDraft = buildLayoutPayload(layoutForm, board?.template);
  const deliveryConfigured = isDeliveryConfigured(deliveryDraft);
  const layoutConfigured = isLayoutConfigured(layoutDraft);
  const layoutFieldCount = countFilledLayoutFields(layoutDraft);
  const filteredPartners =
    searchQuery.trim() === ""
      ? partners
      : partners.filter((partner) => {
          const haystack = [
            partner.fandom ?? "",
            partner.name,
            partner.link,
          ]
            .join(" ")
            .toLowerCase();
          return haystack.includes(searchQuery.trim().toLowerCase());
        });

  const shellStatus = getPartnerBoardShellStatus({
    authState,
    board,
    deliveryConfigured,
    hasLoadedAttempt,
    lastSyncedAt,
    layoutConfigured,
    loading,
    partnerCount: partners.length,
    selectedGuildID,
  });

  let workspaceState: WorkspaceState = "ready";
  if (authState === "checking") {
    workspaceState = "checking";
  } else if (authState !== "signed_in") {
    workspaceState = "auth_required";
  } else if (selectedGuildID.trim() === "") {
    workspaceState = "server_required";
  } else if (loading && board === null) {
    workspaceState = "loading";
  } else if (board === null) {
    workspaceState = "unavailable";
  }

  function resetWorkspace() {
    setBoard(null);
    setDeliveryForm(initialDeliveryForm);
    setLayoutForm(initialLayoutForm);
    setEntryForm(initialEntryForm);
    setSearchQuery("");
    setPendingDeleteName(null);
    setEditingPartnerName("");
    setDrawerMode("create");
    setIsDrawerOpen(false);
    setLoading(false);
    setBusyLabel("");
    setLastLoadedAt(null);
    setLastSyncedAt(null);
    setHasLoadedAttempt(false);
    setNotice(null);
  }

  function applyBoard(nextBoard: PartnerBoardConfig) {
    const nextForms = formsFromBoard(nextBoard);
    setBoard(nextBoard);
    setDeliveryForm(nextForms.deliveryForm);
    setLayoutForm(nextForms.layoutForm);
    setLastLoadedAt(Date.now());
    setHasLoadedAttempt(true);
  }

  async function loadBoardData(successMessage?: string) {
    if (!canManageGuild) {
      return;
    }

    setLoading(true);
    setBusyLabel("Refreshing board data");

    try {
      const response = await client.getPartnerBoard(selectedGuildID.trim());
      applyBoard(response.partner_board);
      if (successMessage) {
        setNotice({
          tone: "success",
          message: successMessage,
        });
      } else {
        setNotice(null);
      }
    } catch (error) {
      setBoard(null);
      setHasLoadedAttempt(true);
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
      setBusyLabel("");
    }
  }

  useEffect(() => {
    if (authState !== "signed_in" || selectedGuildID.trim() === "") {
      resetWorkspace();
      return;
    }

    let cancelled = false;

    async function autoLoadBoard() {
      setLoading(true);
      setBusyLabel("Loading board data");

      try {
        const response = await client.getPartnerBoard(selectedGuildID.trim());
        if (cancelled) {
          return;
        }
        applyBoard(response.partner_board);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setBoard(null);
        setHasLoadedAttempt(true);
        setNotice({
          tone: "error",
          message: formatError(error),
        });
      } finally {
        if (!cancelled) {
          setLoading(false);
          setBusyLabel("");
        }
      }
    }

    void autoLoadBoard();

    return () => {
      cancelled = true;
    };
  }, [authState, client, selectedGuildID]);

  function setDeliveryFormField(
    field: keyof DeliveryFormState,
    value: string,
  ) {
    setDeliveryForm((currentValue) => ({
      ...currentValue,
      [field]: value,
    }));
  }

  function setLayoutFormField(field: keyof LayoutFormState, value: string) {
    setLayoutForm((currentValue) => ({
      ...currentValue,
      [field]: value,
    }));
  }

  function setEntryFormField(field: keyof EntryFormState, value: string) {
    setEntryForm((currentValue) => ({
      ...currentValue,
      [field]: value,
    }));
  }

  function openCreateEntryDrawer() {
    setDrawerMode("create");
    setEditingPartnerName("");
    setEntryForm(initialEntryForm);
    setIsDrawerOpen(true);
  }

  function openEditEntryDrawer(partner: PartnerEntryConfig) {
    setDrawerMode("edit");
    setEditingPartnerName(partner.name);
    setEntryForm({
      fandom: partner.fandom ?? "",
      name: partner.name,
      link: partner.link,
    });
    setIsDrawerOpen(true);
  }

  function closeEntryDrawer() {
    setIsDrawerOpen(false);
    setDrawerMode("create");
    setEditingPartnerName("");
    setEntryForm(initialEntryForm);
  }

  async function saveDelivery() {
    if (!canManageGuild) {
      return;
    }

    const validationError = validateDeliveryForm(deliveryForm);
    if (validationError !== null) {
      setNotice({
        tone: "error",
        message: validationError,
      });
      return;
    }

    setLoading(true);
    setBusyLabel("Saving posting destination");

    try {
      await client.setPartnerBoardTarget(selectedGuildID.trim(), deliveryDraft);
      await loadBoardData("Posting destination updated.");
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
      setLoading(false);
      setBusyLabel("");
    }
  }

  async function saveLayout() {
    if (!canManageGuild) {
      return;
    }

    setLoading(true);
    setBusyLabel("Saving layout settings");

    try {
      await client.setPartnerBoardTemplate(selectedGuildID.trim(), layoutDraft);
      await loadBoardData("Layout updated.");
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
      setLoading(false);
      setBusyLabel("");
    }
  }

  async function saveEntry() {
    if (!canManageGuild) {
      return;
    }

    const validationError = validateEntryForm(entryForm);
    if (validationError !== null) {
      setNotice({
        tone: "error",
        message: validationError,
      });
      return;
    }

    setLoading(true);
    setBusyLabel(drawerMode === "edit" ? "Saving partner entry" : "Adding partner entry");

    try {
      if (drawerMode === "edit") {
        await client.updatePartner(selectedGuildID.trim(), editingPartnerName, {
          fandom: entryForm.fandom.trim(),
          link: entryForm.link.trim(),
          name: entryForm.name.trim(),
        });
        closeEntryDrawer();
        await loadBoardData("Partner entry updated.");
        return;
      }

      await client.createPartner(selectedGuildID.trim(), {
        fandom: entryForm.fandom.trim(),
        link: entryForm.link.trim(),
        name: entryForm.name.trim(),
      });
      closeEntryDrawer();
      await loadBoardData("Partner entry added.");
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
      setLoading(false);
      setBusyLabel("");
    }
  }

  async function confirmDeleteEntry(partnerName: string) {
    if (!canManageGuild) {
      return;
    }

    setLoading(true);
    setBusyLabel("Removing partner entry");

    try {
      await client.deletePartner(selectedGuildID.trim(), partnerName);
      setPendingDeleteName(null);
      if (editingPartnerName === partnerName) {
        closeEntryDrawer();
      }
      await loadBoardData("Partner entry removed.");
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
      setLoading(false);
      setBusyLabel("");
    }
  }

  async function refreshBoard() {
    await loadBoardData("Partner Board refreshed.");
  }

  async function syncBoard() {
    if (!canManageGuild) {
      return;
    }

    setLoading(true);
    setBusyLabel("Syncing to Discord");

    try {
      await client.syncPartnerBoard(selectedGuildID.trim());
      setLastSyncedAt(Date.now());
      setNotice({
        tone: "success",
        message: "Partner Board synced to Discord.",
      });
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
      setBusyLabel("");
    }
  }

  return (
    <PartnerBoardContext.Provider
      value={{
        board,
        busyLabel,
        deliveryConfigured,
        deliveryForm,
        drawerMode,
        entryForm,
        filteredPartners,
        hasLoadedAttempt,
        isDrawerOpen,
        lastLoadedAt,
        lastSyncedAt,
        layoutConfigured,
        layoutFieldCount,
        layoutForm,
        loading,
        notice,
        partners,
        pendingDeleteName,
        searchQuery,
        shellStatus,
        workspaceState,
        clearNotice: () => setNotice(null),
        closeEntryDrawer,
        confirmDeleteEntry,
        openCreateEntryDrawer,
        openEditEntryDrawer,
        refreshBoard,
        saveDelivery,
        saveEntry,
        saveLayout,
        setDeliveryFormField,
        setEntryFormField,
        setLayoutFormField,
        setSearchQuery,
        summarizePostingDestination: summarizePostingDestination(deliveryDraft),
        syncBoard,
        toggleDeleteEntry: setPendingDeleteName,
      }}
    >
      {children}
    </PartnerBoardContext.Provider>
  );
}

export function usePartnerBoard() {
  const context = useContext(PartnerBoardContext);
  if (context === null) {
    throw new Error("usePartnerBoard must be used inside PartnerBoardProvider");
  }
  return context;
}

function validateEntryForm(form: EntryFormState): string | null {
  if (form.name.trim() === "") {
    return "Partner name is required.";
  }

  if (form.link.trim() === "") {
    return "Invite link is required.";
  }

  return null;
}

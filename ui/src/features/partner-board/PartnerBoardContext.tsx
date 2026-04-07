/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
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
import {
  readPartnerBoardCache,
  writePartnerBoardCache,
} from "./cache";

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
  deliveryDirty: boolean;
  deliveryForm: DeliveryFormState;
  drawerMode: EntryDrawerMode;
  entryDirty: boolean;
  entryForm: EntryFormState;
  filteredPartners: PartnerEntryConfig[];
  hasLoadedAttempt: boolean;
  isDrawerOpen: boolean;
  lastLoadedAt: number | null;
  lastSyncedAt: number | null;
  layoutConfigured: boolean;
  layoutDirty: boolean;
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
  resetDeliveryForm: () => void;
  resetEntryForm: () => void;
  resetLayoutForm: () => void;
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
  const {
    authState,
    baseUrl,
    canEditSelectedGuild,
    canReadSelectedGuild,
    client,
    selectedGuildID,
  } =
    useDashboardSession();
  const normalizedGuildID = selectedGuildID.trim();
  const initialCachedEntry =
    authState === "signed_in" && normalizedGuildID !== ""
      ? readPartnerBoardCache(baseUrl, normalizedGuildID)
      : null;
  const initialCachedBoard = initialCachedEntry?.board ?? null;
  const initialForms = formsFromBoard(initialCachedBoard);
  const [board, setBoard] = useState<PartnerBoardConfig | null>(initialCachedBoard);
  const [deliveryForm, setDeliveryForm] = useState(initialForms.deliveryForm);
  const [layoutForm, setLayoutForm] = useState(initialForms.layoutForm);
  const [entryForm, setEntryForm] = useState(initialEntryForm);
  const [searchQuery, setSearchQuery] = useState("");
  const [pendingDeleteName, setPendingDeleteName] = useState<string | null>(null);
  const [editingPartnerName, setEditingPartnerName] = useState("");
  const [drawerMode, setDrawerMode] = useState<EntryDrawerMode>("create");
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [loading, setLoading] = useState(false);
  const [busyLabel, setBusyLabel] = useState("");
  const [lastLoadedAt, setLastLoadedAt] = useState<number | null>(
    initialCachedEntry?.fetchedAt ?? null,
  );
  const [lastSyncedAt, setLastSyncedAt] = useState<number | null>(null);
  const [hasLoadedAttempt, setHasLoadedAttempt] = useState(initialCachedBoard !== null);
  const boardRef = useRef<PartnerBoardConfig | null>(initialCachedBoard);
  const deliveryFormRef = useRef(initialForms.deliveryForm);
  const layoutFormRef = useRef(initialForms.layoutForm);

  const partners = board?.partners ?? [];
  const boardForms = formsFromBoard(board);
  const deliveryDraft = buildDeliveryPayload(deliveryForm);
  const layoutDraft = buildLayoutPayload(layoutForm, board?.template);
  const deliveryConfigured = isDeliveryConfigured(deliveryDraft);
  const deliveryDirty = !areDeliveryFormsEqual(deliveryForm, boardForms.deliveryForm);
  const layoutConfigured = isLayoutConfigured(layoutDraft);
  const layoutDirty = !areLayoutFormsEqual(layoutForm, boardForms.layoutForm);
  const layoutFieldCount = countFilledLayoutFields(layoutDraft);
  const entryBaseline = getEntryBaseline(board, drawerMode, editingPartnerName);
  const entryDirty = !areEntryFormsEqual(entryForm, entryBaseline);
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
  } else if (normalizedGuildID === "") {
    workspaceState = "server_required";
  } else if (loading && board === null) {
    workspaceState = "loading";
  } else if (board === null) {
    workspaceState = "unavailable";
  }

  const resetTransientWorkspaceState = useCallback(() => {
    setEntryForm(initialEntryForm);
    setSearchQuery("");
    setPendingDeleteName(null);
    setEditingPartnerName("");
    setDrawerMode("create");
    setIsDrawerOpen(false);
  }, []);

  const resetWorkspace = useCallback(() => {
    setBoard(null);
    setDeliveryForm(initialDeliveryForm);
    setLayoutForm(initialLayoutForm);
    resetTransientWorkspaceState();
    setLoading(false);
    setBusyLabel("");
    setLastLoadedAt(null);
    setLastSyncedAt(null);
    setHasLoadedAttempt(false);
    setNotice(null);
  }, [resetTransientWorkspaceState]);

  useEffect(() => {
    boardRef.current = board;
  }, [board]);

  useEffect(() => {
    deliveryFormRef.current = deliveryForm;
  }, [deliveryForm]);

  useEffect(() => {
    layoutFormRef.current = layoutForm;
  }, [layoutForm]);

  const applyBoard = useCallback((
    nextBoard: PartnerBoardConfig,
    options: {
      cache?: boolean;
      fetchedAt?: number;
      preserveDeliveryDraft?: boolean;
      preserveLayoutDraft?: boolean;
    } = {},
  ) => {
    const fetchedAt = options.fetchedAt ?? Date.now();
    const previousForms = formsFromBoard(boardRef.current);
    const nextForms = formsFromBoard(nextBoard);
    if (options.cache !== false && normalizedGuildID !== "") {
      writePartnerBoardCache(baseUrl, normalizedGuildID, nextBoard, fetchedAt);
    }
    const preserveDeliveryDraft =
      options.preserveDeliveryDraft !== false &&
      !areDeliveryFormsEqual(
        deliveryFormRef.current,
        previousForms.deliveryForm,
      );
    const preserveLayoutDraft =
      options.preserveLayoutDraft !== false &&
      !areLayoutFormsEqual(layoutFormRef.current, previousForms.layoutForm);
    boardRef.current = nextBoard;
    setBoard(nextBoard);
    setDeliveryForm(
      preserveDeliveryDraft ? deliveryFormRef.current : nextForms.deliveryForm,
    );
    setLayoutForm(
      preserveLayoutDraft ? layoutFormRef.current : nextForms.layoutForm,
    );
    setLastLoadedAt(fetchedAt);
    setHasLoadedAttempt(true);
  }, [baseUrl, normalizedGuildID]);

  async function loadBoardData(options: {
    preservePendingState?: boolean;
    successMessage?: string;
  } = {}) {
    if (!canReadSelectedGuild || normalizedGuildID === "") {
      return;
    }

    const preservePendingState = options.preservePendingState ?? false;
    const cachedEntry = readPartnerBoardCache(baseUrl, normalizedGuildID);
    if (!preservePendingState && cachedEntry !== null) {
      applyBoard(cachedEntry.board, {
        cache: false,
        fetchedAt: cachedEntry.fetchedAt,
      });
      setLoading(false);
    } else if (!preservePendingState) {
      setLoading(true);
    }
    if (!preservePendingState) {
      setBusyLabel("");
    }

    try {
      const response = await client.getPartnerBoard(normalizedGuildID);
      applyBoard(response.partner_board);
      if (options.successMessage) {
        setNotice({
          tone: "success",
          message: options.successMessage,
        });
      } else {
        setNotice(null);
      }
    } catch (error) {
      const fallbackEntry = readPartnerBoardCache(baseUrl, normalizedGuildID);
      if (fallbackEntry !== null) {
        applyBoard(fallbackEntry.board, {
          cache: false,
          fetchedAt: fallbackEntry.fetchedAt,
        });
        setNotice(null);
      } else {
        setBoard(null);
        setHasLoadedAttempt(true);
        setNotice({
          tone: "error",
          message: formatError(error),
        });
      }
    } finally {
      setLoading(false);
      setBusyLabel("");
    }
  }

  useEffect(() => {
    if (authState !== "signed_in" || normalizedGuildID === "") {
      resetWorkspace();
      return;
    }

    resetTransientWorkspaceState();
    const cachedEntry = readPartnerBoardCache(baseUrl, normalizedGuildID);
    if (cachedEntry !== null) {
      applyBoard(cachedEntry.board, {
        cache: false,
        fetchedAt: cachedEntry.fetchedAt,
      });
      setLoading(false);
    } else {
      setBoard(null);
      setDeliveryForm(initialDeliveryForm);
      setLayoutForm(initialLayoutForm);
      setLastLoadedAt(null);
      setHasLoadedAttempt(false);
      setLoading(true);
    }
    setNotice(null);

    let cancelled = false;

    async function autoLoadBoard() {
      setBusyLabel("");

      try {
        const response = await client.getPartnerBoard(normalizedGuildID);
        if (cancelled) {
          return;
        }
        applyBoard(response.partner_board);
        setNotice(null);
      } catch (error) {
        if (cancelled) {
          return;
        }

        const fallbackEntry = readPartnerBoardCache(baseUrl, normalizedGuildID);
        if (fallbackEntry !== null) {
          applyBoard(fallbackEntry.board, {
            cache: false,
            fetchedAt: fallbackEntry.fetchedAt,
          });
          setNotice(null);
        } else {
          setBoard(null);
          setHasLoadedAttempt(true);
          setNotice({
            tone: "error",
            message: formatError(error),
          });
        }
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
  }, [
    applyBoard,
    authState,
    baseUrl,
    client,
    normalizedGuildID,
    resetTransientWorkspaceState,
    resetWorkspace,
  ]);

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

  const resetDeliveryForm = useCallback(() => {
    setDeliveryForm(formsFromBoard(boardRef.current).deliveryForm);
  }, []);

  const resetLayoutForm = useCallback(() => {
    setLayoutForm(formsFromBoard(boardRef.current).layoutForm);
  }, []);

  const resetEntryForm = useCallback(() => {
    setEntryForm(getEntryBaseline(board, drawerMode, editingPartnerName));
  }, [board, drawerMode, editingPartnerName]);

  async function saveDelivery() {
    if (!canEditSelectedGuild) {
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
      const response = await client.setPartnerBoardTarget(
        normalizedGuildID,
        deliveryDraft,
      );
      applyBoard(
        mergeBoardConfig(boardRef.current, {
          target: response.target,
        }),
        {
          preserveDeliveryDraft: false,
        },
      );
      setNotice({
        tone: "success",
        message: "Posting destination updated.",
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

  async function saveLayout() {
    if (!canEditSelectedGuild) {
      return;
    }

    setLoading(true);
    setBusyLabel("Saving layout settings");

    try {
      const response = await client.setPartnerBoardTemplate(
        normalizedGuildID,
        layoutDraft,
      );
      applyBoard(
        mergeBoardConfig(boardRef.current, {
          template: response.template,
        }),
        {
          preserveLayoutDraft: false,
        },
      );
      setNotice({
        tone: "success",
        message: "Layout updated.",
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

  async function saveEntry() {
    if (!canEditSelectedGuild) {
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
        const response = await client.updatePartner(normalizedGuildID, editingPartnerName, {
          fandom: entryForm.fandom.trim(),
          link: entryForm.link.trim(),
          name: entryForm.name.trim(),
        });
        closeEntryDrawer();
        applyBoard(
          mergeBoardConfig(boardRef.current, {
            partners: sortPartnersByName([
              ...(boardRef.current?.partners ?? []).filter(
                (partner) => partner.name !== editingPartnerName,
              ),
              response.partner,
            ]),
          }),
        );
        setNotice({
          tone: "success",
          message: "Partner entry updated.",
        });
        return;
      }

      const response = await client.createPartner(normalizedGuildID, {
        fandom: entryForm.fandom.trim(),
        link: entryForm.link.trim(),
        name: entryForm.name.trim(),
      });
      closeEntryDrawer();
      applyBoard(
        mergeBoardConfig(boardRef.current, {
          partners: sortPartnersByName([
            ...(boardRef.current?.partners ?? []),
            response.partner,
          ]),
        }),
      );
      setNotice({
        tone: "success",
        message: "Partner entry added.",
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

  async function confirmDeleteEntry(partnerName: string) {
    if (!canEditSelectedGuild) {
      return;
    }

    setLoading(true);
    setBusyLabel("Removing partner entry");

    try {
      await client.deletePartner(normalizedGuildID, partnerName);
      setPendingDeleteName(null);
      if (editingPartnerName === partnerName) {
        closeEntryDrawer();
      }
      applyBoard(
        mergeBoardConfig(boardRef.current, {
          partners: (boardRef.current?.partners ?? []).filter(
            (partner) => partner.name !== partnerName,
          ),
        }),
      );
      setNotice({
        tone: "success",
        message: "Partner entry removed.",
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

  async function refreshBoard() {
    setLoading(true);
    setBusyLabel("Refreshing Partner Board");
    await loadBoardData({
      preservePendingState: true,
      successMessage: "Partner Board refreshed.",
    });
  }

  async function syncBoard() {
    if (!canEditSelectedGuild) {
      return;
    }

    setLoading(true);
    setBusyLabel("Syncing to Discord");

    try {
      await client.syncPartnerBoard(normalizedGuildID);
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
        deliveryDirty,
        deliveryForm,
        drawerMode,
        entryDirty,
        entryForm,
        filteredPartners,
        hasLoadedAttempt,
        isDrawerOpen,
        lastLoadedAt,
        lastSyncedAt,
        layoutConfigured,
        layoutDirty,
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
        resetDeliveryForm,
        resetEntryForm,
        resetLayoutForm,
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

function areDeliveryFormsEqual(
  left: DeliveryFormState,
  right: DeliveryFormState,
) {
  return (
    left.type === right.type &&
    left.messageID === right.messageID &&
    left.webhookURL === right.webhookURL &&
    left.channelID === right.channelID
  );
}

function areLayoutFormsEqual(left: LayoutFormState, right: LayoutFormState) {
  return (
    left.title === right.title &&
    left.intro === right.intro &&
    left.sectionHeaderTemplate === right.sectionHeaderTemplate &&
    left.lineTemplate === right.lineTemplate &&
    left.emptyStateText === right.emptyStateText
  );
}

function areEntryFormsEqual(left: EntryFormState, right: EntryFormState) {
  return (
    left.fandom === right.fandom &&
    left.name === right.name &&
    left.link === right.link
  );
}

function getEntryBaseline(
  board: PartnerBoardConfig | null,
  drawerMode: EntryDrawerMode,
  editingPartnerName: string,
): EntryFormState {
  if (drawerMode !== "edit" || editingPartnerName.trim() === "") {
    return initialEntryForm;
  }

  const partner = board?.partners?.find(
    (entry) => entry.name === editingPartnerName,
  );
  if (partner === undefined) {
    return initialEntryForm;
  }

  return {
    fandom: partner.fandom ?? "",
    name: partner.name,
    link: partner.link,
  };
}

function mergeBoardConfig(
  currentBoard: PartnerBoardConfig | null,
  patch: Partial<PartnerBoardConfig>,
): PartnerBoardConfig {
  return {
    ...(currentBoard ?? {}),
    ...patch,
    target: patch.target ?? currentBoard?.target,
    template: patch.template ?? currentBoard?.template,
    partners: patch.partners ?? currentBoard?.partners ?? [],
  };
}

function sortPartnersByName(partners: PartnerEntryConfig[]) {
  return [...partners].sort((left, right) => left.name.localeCompare(right.name));
}

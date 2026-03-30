import type { PartnerBoardConfig } from "../../api/control";

interface CachedPartnerBoard {
  board: PartnerBoardConfig;
  fetchedAt: number;
}

const partnerBoardCache = new Map<string, CachedPartnerBoard>();

export function readPartnerBoardCache(
  baseUrl: string,
  guildID: string,
) {
  return partnerBoardCache.get(buildPartnerBoardCacheKey(baseUrl, guildID)) ?? null;
}

export function peekPartnerBoard(
  baseUrl: string,
  guildID: string,
) {
  return readPartnerBoardCache(baseUrl, guildID)?.board ?? null;
}

export function writePartnerBoardCache(
  baseUrl: string,
  guildID: string,
  board: PartnerBoardConfig,
  fetchedAt = Date.now(),
) {
  partnerBoardCache.set(buildPartnerBoardCacheKey(baseUrl, guildID), {
    board,
    fetchedAt,
  });
}

export function resetPartnerBoardCache() {
  partnerBoardCache.clear();
}

function buildPartnerBoardCacheKey(baseUrl: string, guildID: string) {
  return `${baseUrl}::${guildID.trim()}`;
}

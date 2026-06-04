import { render, screen, waitFor } from "@testing-library/react";
import { TranscriptViewer } from "./TranscriptViewer";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe("TranscriptViewer", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    // Default mock implementation
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => [],
    });
  });

  it("renders error when no url is provided", () => {
    // Override window location search
    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      value: { ...originalLocation, search: "" },
      writable: true,
    });

    render(<TranscriptViewer />);
    
    expect(screen.getByText("No transcript URL provided.")).toBeInTheDocument();
  });

  it("fetches and renders transcript messages", async () => {
    Object.defineProperty(window, "location", {
      value: { search: "?url=https://example.com/transcript.json" },
      writable: true,
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [
        {
          id: "1",
          author: { id: "123", username: "TestUser", avatar: "abc" },
          content: "Hello world!",
          timestamp: new Date().toISOString(),
        }
      ],
    });

    render(<TranscriptViewer />);
    
    expect(screen.getByText("Loading transcript...")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Hello world!")).toBeInTheDocument();
    });
    expect(screen.getByText("TestUser")).toBeInTheDocument();
  });

  it("shows error if fetch fails", async () => {
    Object.defineProperty(window, "location", {
      value: { search: "?url=https://example.com/transcript.json" },
      writable: true,
    });

    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    render(<TranscriptViewer />);

    await waitFor(() => {
      expect(screen.getByText("Network error")).toBeInTheDocument();
    });
  });
});

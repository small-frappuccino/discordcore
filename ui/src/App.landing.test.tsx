import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import App from "./App";

describe("landing route", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    window.history.replaceState({}, "", "/");
  });

  it("keeps the root path on the landing page instead of redirecting to /manage", async () => {
    const fetchMock = vi.fn(async () => new Response("", { status: 401 }));
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/");

    render(<App />);

    await screen.findByRole("button", { name: "Login with Discord" });

    await waitFor(() => {
      expect(window.location.pathname).toBe("/");
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:3000/auth/me",
      expect.objectContaining({
        credentials: "include",
        method: "GET",
      }),
    );
  });
});

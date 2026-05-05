package control

import (
	"bytes"
	"net/http"
	"time"
)

const controlLandingHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Discordcore Control</title>
    <style>
      :root {
        color-scheme: dark;
        --bg-canvas: #171318;
        --bg-canvas-alt: #211c22;
        --panel-base: rgba(35, 29, 33, 0.88);
        --border-soft: rgba(244, 231, 220, 0.14);
        --text-main: #f7efe6;
        --text-muted: #d8c8bb;
        --accent-primary: #dbc7b2;
        --accent-secondary: #e8ac98;
        --accent-danger: #d98b89;
        --accent-moss: #9caf8c;
        --shadow-strong: 0 32px 72px rgba(11, 8, 11, 0.44);
        --radius-pill: 999px;
        --font-body:
          "Aptos",
          "Segoe UI Variable Text",
          "Segoe UI",
          "Trebuchet MS",
          sans-serif;
      }

      * {
        box-sizing: border-box;
      }

      body {
        margin: 0;
        min-height: 100vh;
        color: var(--text-main);
        font-family: var(--font-body);
        background:
          radial-gradient(circle at 14% 20%, rgba(219, 199, 178, 0.14), transparent 28%),
          radial-gradient(circle at 82% 18%, rgba(232, 172, 152, 0.16), transparent 24%),
          radial-gradient(circle at 72% 80%, rgba(156, 175, 140, 0.11), transparent 26%),
          linear-gradient(180deg, #110f13 0%, var(--bg-canvas) 42%, var(--bg-canvas-alt) 100%);
      }

      body::before {
        content: "";
        position: fixed;
        inset: 0;
        pointer-events: none;
        background:
          radial-gradient(circle at 22% 72%, rgba(219, 199, 178, 0.08), transparent 20%),
          radial-gradient(circle at 78% 62%, rgba(232, 172, 152, 0.08), transparent 18%);
      }

      .shell {
        padding: 24px;
      }

      .topbar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
        padding: 16px 18px;
        border-radius: var(--radius-pill);
        border: 1px solid var(--border-soft);
        background:
          linear-gradient(180deg, rgba(255, 255, 255, 0.03), transparent 54%),
          var(--panel-base);
        box-shadow: var(--shadow-strong);
        backdrop-filter: blur(18px);
      }

      .brand {
        width: 62px;
        height: 62px;
        display: grid;
        place-items: center;
        padding: 7px;
        border-radius: 50%;
        overflow: hidden;
        background: rgba(11, 8, 11, 0.92);
        box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
        flex-shrink: 0;
      }

      .brand img {
        width: 100%;
        height: 100%;
        display: block;
        border-radius: 50%;
        object-fit: cover;
        object-position: center;
      }

      .actions {
        display: flex;
        align-items: center;
        justify-content: flex-end;
        gap: 12px;
        flex-wrap: wrap;
      }

      .session-panel {
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        gap: 10px;
        flex: 1;
      }

      .button {
        appearance: none;
        min-width: 180px;
        padding: 12px 18px;
        border-radius: var(--radius-pill);
        border: 1px solid rgba(216, 200, 187, 0.26);
        background: rgba(255, 255, 255, 0.04);
        color: var(--text-main);
        font: inherit;
        font-weight: 700;
        cursor: pointer;
        transition:
          border-color 160ms ease,
          background-color 160ms ease,
          transform 160ms ease;
      }

      .button:hover:not(:disabled) {
        transform: translateY(-1px);
        border-color: rgba(232, 172, 152, 0.42);
      }

      .button:disabled {
        cursor: not-allowed;
        opacity: 0.62;
      }

      .button-primary {
        border-color: rgba(232, 172, 152, 0.34);
        background: rgba(232, 172, 152, 0.14);
      }

      .button-secondary {
        border-color: rgba(156, 175, 140, 0.34);
        background: rgba(156, 175, 140, 0.12);
      }

      .button-ghost {
        border-color: rgba(217, 139, 137, 0.34);
        background: rgba(217, 139, 137, 0.08);
        color: #fff0ee;
      }

      .is-hidden {
        display: none;
      }

      @media (max-width: 720px) {
        .shell {
          padding: 16px;
        }

        .topbar {
          flex-direction: column;
          align-items: flex-start;
        }

        .session-panel {
          width: 100%;
          align-items: flex-start;
        }

        .actions {
          width: 100%;
          justify-content: flex-start;
        }

        .button {
          width: 100%;
        }
      }
    </style>
  </head>
  <body>
    <header class="shell">
      <div class="topbar">
        <div class="brand" aria-hidden="true">
          <img src="/manage/brand/discordmain.webp" alt="" />
        </div>

        <div class="session-panel">
          <div class="actions">
            <button id="login-button" class="button button-primary" type="button">
              Login com Discord
            </button>
            <button id="dashboard-button" class="button button-secondary" type="button">
              Dashboard
            </button>
            <button id="logout-button" class="button button-ghost is-hidden" type="button">
              Logout
            </button>
          </div>
        </div>
      </div>
    </header>

    <script>
      (() => {
        const loginButton = document.getElementById("login-button");
        const dashboardButton = document.getElementById("dashboard-button");
        const logoutButton = document.getElementById("logout-button");
        let csrfToken = "";
        let loginURL = "/auth/discord/login?next=%2Fmanage%2F";
        let dashboardURL = "/manage/";

        function hide(element, hidden) {
          element.classList.toggle("is-hidden", hidden);
        }

        function showSignedOut(oauthAvailable, nextLoginURL, nextDashboardURL) {
          csrfToken = "";
          loginURL = nextLoginURL || "/auth/discord/login?next=%2Fmanage%2F";
          dashboardURL = nextDashboardURL || "/manage/";
          hide(loginButton, false);
          hide(dashboardButton, false);
          hide(logoutButton, true);
          loginButton.disabled = !oauthAvailable;
          loginButton.textContent = oauthAvailable ? "Login com Discord" : "Discord indisponível";
        }

        function showSignedIn(token, nextDashboardURL) {
          csrfToken = token;
          dashboardURL = nextDashboardURL || "/manage/";
          hide(loginButton, true);
          hide(dashboardButton, false);
          hide(logoutButton, false);
          loginButton.disabled = false;
          loginButton.textContent = "Login com Discord";
        }

        async function refreshSession() {
          try {
            const response = await fetch("/auth/discord/status?next=%2Fmanage%2F", {
              method: "GET",
              credentials: "include"
            });
            if (!response.ok) {
              throw new Error("status probe failed");
            }

            const payload = await response.json();
            const oauthAvailable = Boolean(payload.oauth_configured);
            const authenticated = Boolean(payload.authenticated);
            const nextLoginURL = String(payload.login_url || "").trim();
            const nextDashboardURL = String(payload.dashboard_url || "").trim();

            if (authenticated) {
              showSignedIn(String(payload.csrf_token || "").trim(), nextDashboardURL);
              return;
            }

            showSignedOut(oauthAvailable, nextLoginURL, nextDashboardURL);
          } catch {
            showSignedOut(true, "", "");
          }
        }

        loginButton.addEventListener("click", () => {
          if (loginButton.disabled) {
            return;
          }
          window.location.assign(loginURL);
        });

        dashboardButton.addEventListener("click", () => {
          window.location.assign(dashboardURL);
        });

        logoutButton.addEventListener("click", async () => {
          if (csrfToken === "") {
            await refreshSession();
          }
          if (csrfToken === "") {
            return;
          }

          try {
            await fetch("/auth/logout", {
              method: "POST",
              credentials: "include",
              headers: {
                "X-CSRF-Token": csrfToken
              }
            });
          } finally {
            showSignedOut(true, "");
          }
        });

        refreshSession();
      })();
    </script>
  </body>
</html>
`

type landingHandler struct{}

func newLandingHandler() http.Handler {
	return landingHandler{}
}

func (landingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader([]byte(controlLandingHTML)))
}

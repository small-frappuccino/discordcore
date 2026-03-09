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
        --bg-canvas: #09111d;
        --bg-canvas-alt: #0f1c2d;
        --panel-base: rgba(12, 22, 37, 0.88);
        --border-soft: rgba(143, 170, 196, 0.22);
        --text-main: #f4f7fb;
        --text-muted: #b3c2d5;
        --accent-primary: #7bd8c7;
        --accent-secondary: #f6bd74;
        --accent-danger: #ff8a7c;
        --shadow-strong: 0 32px 72px rgba(2, 8, 18, 0.45);
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
          radial-gradient(circle at 14% 20%, rgba(123, 216, 199, 0.12), transparent 28%),
          radial-gradient(circle at 82% 18%, rgba(246, 189, 116, 0.13), transparent 24%),
          radial-gradient(circle at 72% 80%, rgba(255, 138, 124, 0.11), transparent 26%),
          linear-gradient(180deg, #07101b 0%, var(--bg-canvas) 42%, var(--bg-canvas-alt) 100%);
      }

      body::before {
        content: "";
        position: fixed;
        inset: 0;
        pointer-events: none;
        background:
          radial-gradient(circle at 22% 72%, rgba(123, 216, 199, 0.08), transparent 20%),
          radial-gradient(circle at 78% 62%, rgba(246, 189, 116, 0.08), transparent 18%);
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
        flex-shrink: 0;
      }

      .brand svg {
        width: 62px;
        height: 62px;
        display: block;
      }

      .actions {
        display: flex;
        align-items: center;
        justify-content: flex-end;
        gap: 12px;
        flex-wrap: wrap;
      }

      .button {
        appearance: none;
        min-width: 180px;
        padding: 12px 18px;
        border-radius: var(--radius-pill);
        border: 1px solid rgba(246, 189, 116, 0.34);
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
        border-color: rgba(123, 216, 199, 0.42);
      }

      .button:disabled {
        cursor: not-allowed;
        opacity: 0.62;
      }

      .button-primary {
        border-color: rgba(123, 216, 199, 0.32);
        background: rgba(123, 216, 199, 0.12);
      }

      .button-secondary {
        border-color: rgba(246, 189, 116, 0.34);
        background: rgba(246, 189, 116, 0.1);
      }

      .button-ghost {
        border-color: rgba(255, 138, 124, 0.34);
        background: rgba(255, 138, 124, 0.08);
        color: #ffe1db;
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
        <div class="brand" aria-label="Bot icon">
          <svg viewBox="0 0 96 96" role="img" aria-hidden="true">
            <defs>
              <linearGradient id="landing-icon-shell" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" stop-color="#7bd8c7" />
                <stop offset="100%" stop-color="#f6bd74" />
              </linearGradient>
            </defs>
            <circle cx="48" cy="48" r="44" fill="#07111d" stroke="#f6bd74" stroke-width="3" />
            <path d="M29 22c0-6 4-10 10-10 5 0 9 3 10 8 1-5 5-8 10-8 6 0 10 4 10 10v14H29V22Z" fill="url(#landing-icon-shell)" />
            <rect x="24" y="30" width="48" height="40" rx="20" fill="#f7f3ec" />
            <circle cx="38" cy="47" r="6" fill="#f08a6a" />
            <circle cx="58" cy="47" r="6" fill="#f08a6a" />
            <circle cx="38" cy="47" r="2.5" fill="#6f432a" />
            <circle cx="58" cy="47" r="2.5" fill="#6f432a" />
            <path d="M39 60c4 2.8 14 2.8 18 0" fill="none" stroke="#6f432a" stroke-width="3" stroke-linecap="round" />
          </svg>
        </div>

        <div class="actions">
          <button id="login-button" class="button button-primary" type="button">
            Login com Discord
          </button>
          <button id="dashboard-button" class="button button-secondary is-hidden" type="button">
            Dashboard
          </button>
          <button id="logout-button" class="button button-ghost is-hidden" type="button">
            Logout
          </button>
        </div>
      </div>
    </header>

    <script>
      (() => {
        const loginButton = document.getElementById("login-button");
        const dashboardButton = document.getElementById("dashboard-button");
        const logoutButton = document.getElementById("logout-button");
        let csrfToken = "";

        function hide(element, hidden) {
          element.classList.toggle("is-hidden", hidden);
        }

        function showSignedOut(oauthAvailable) {
          csrfToken = "";
          hide(loginButton, false);
          hide(dashboardButton, true);
          hide(logoutButton, true);
          loginButton.disabled = !oauthAvailable;
          loginButton.textContent = oauthAvailable ? "Login com Discord" : "Discord indisponível";
        }

        function showSignedIn(token) {
          csrfToken = token;
          hide(loginButton, true);
          hide(dashboardButton, false);
          hide(logoutButton, false);
          loginButton.disabled = false;
          loginButton.textContent = "Login com Discord";
        }

        async function refreshSession() {
          try {
            const response = await fetch("/auth/me", {
              method: "GET",
              credentials: "include"
            });
            if (response.status === 200) {
              const payload = await response.json();
              showSignedIn(String(payload.csrf_token || "").trim());
              return;
            }
            if (response.status === 503) {
              showSignedOut(false);
              return;
            }
            showSignedOut(true);
          } catch {
            showSignedOut(true);
          }
        }

        loginButton.addEventListener("click", () => {
          window.location.assign("/auth/discord/login?next=%2F");
        });

        dashboardButton.addEventListener("click", () => {
          window.location.assign("/dashboard");
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
            showSignedOut(true);
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

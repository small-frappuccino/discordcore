# UI_RULES.md: Architecture and Interface Engineering Guidelines

This document defines the architectural standards, implementation rules, and absolute constraints for User Interface (UI) development in this repository. The primary objective is to eliminate chronic anti-patterns, guarantee rigorous typing, ensure predictable performance, and enforce strict domain isolation.

## 1. Reference Technology Stack

The use of the following technologies is **mandatory** for their respective purposes. Introducing alternative libraries for these exact purposes requires formal approval.

*   **Styling and Design System:** Tailwind CSS.
*   **Accessibility Primitives:** Radix UI.
*   **Asynchronous State Management and Caching:** React Query (TanStack Query).
*   **Form Management:** React Hook Form.
*   **Data Validation and Parsing:** Zod.
*   **Code Formatting and Quality:** ESLint (strict mode) + Prettier.

## 2. Anti-Pattern Resolution and Structural Rules

### 2.1. End of Primitive Data Fetching
It is **strictly prohibited** to perform network requests (data fetching) and manually manage derived states (e.g., `loading`, `error`, `data`, `saving`) inside `useEffect` or `useState` blocks.
*   **Required Standard:** Use **React Query / TanStack Query** for all API interactions. Delegate cache management, request deduplication, retries, and background revalidation entirely to it.

### 2.2. End of "God Components" (Separation of Concerns)
Massive components that conflate business logic, complex data mutations, and UI rendering are unacceptable.
*   **Required Standard:** Adopt strict separation of concerns. Extract complex logic and API calls into **Custom Hooks**. For larger components, use the *Container/Presenter* pattern, keeping the JSX (the View) focused purely on visual representation and devoid of heavy business logic.

### 2.3. Form Control (End of State Chaos)
Scattering form state across multiple `useState` hooks tied to manually controlled inputs (e.g., `value={state} onChange={e => setState(e.target.value)}`) is prohibited.
*   **Required Standard:** Use *uncontrolled* state-based forms with **React Hook Form**. The form library must operate **inseparably linked to Zod** via resolvers, guaranteeing structural validation before any submission attempt.

### 2.4. Monolith Modularization
Artifacts that centralize distinct domains are prohibited (Single Responsibility Principle).
*   **APIs and Clients:** Gigantic network clients (e.g., `api/control.ts`) must be fragmented and organized by domain or feature.
*   **Global Contexts:** The use of "God Contexts" that mix authentication, routing, network calls, and UI preferences is banned. Segregate and distribute these responsibilities into smaller, highly focused, specific contexts.

### 2.5. Modern Error Handling
The use of blocking native methods (such as `alert()`, `prompt()`, `confirm()`) or simply dumping unhandled errors into the console is **prohibited**.
*   **Required Standard:** Implement React *Error Boundaries* to catch and contain rendering failures either globally or by route. Use non-obtrusive *Toast Notification* components (e.g., Sonner, react-hot-toast) to provide feedback on network mutation success or failure.

### 2.6. Strict Styling
The chaotic mixture of inline styles (`style={{ margin: 10 }}`) and loose CSS classes is **immediately banned**.
*   **Required Standard:** The standard is now the exclusive use of **Tailwind CSS**. The application must consume the strict *Design Tokens* configured in Tailwind for spacing, colors, typography, and borders, ensuring total adherence to the Design System.

## 3. Architecture Guidelines: UI/UX and TypeScript

Interface development must adhere to fundamental pillars to achieve robustness and technical excellence.

### 3.1. Typing and Data Architecture

*   **Parse, Don't Validate:** Any data received from APIs or user inputs must be processed at the application's edge (network boundary). Use **Zod** to "parse" and transform unknown data into concrete, guaranteed domain types before they ever reach the UI layer.
*   **Single Source of Truth:** State must exist in only one place. Avoid mirroring or duplicating state. Derived data from existing states must be calculated synchronously during the View's rendering (or via memoized *selectors*). This prevents state desynchronization (*zombie data*).
*   **Composition over Inheritance:** Favor composing simple components into complex interfaces, utilizing dependency injection via *props* (`children`, *render props*) and specialized *hooks*. Avoid creating monolithic components with deep prop drilling or excessive props ("boolean traps").

### 3.2. UI Quality and Accessibility (a11y)

*   **Accessibility by Default:** Accessibility is not an optional requirement. The adoption of natively accessible UI primitives via **Radix UI** is imperative for modals, dropdowns, popovers, accordions, etc. Expected behaviors such as natural keyboard navigation and focus management, alongside correct ARIA tags and properties, must be intrinsic to all interactive components.
*   **Design Tokens and Tailwind:** Strictly consume the Tailwind CSS design tokens defined in the configuration. Hardcoded hexadecimal colors and ad-hoc margins/paddings (outside the scale) will be rejected.
*   **Latency Feedback (UX):** Primary interactions must respond in under **100ms** (Interaction to Next Paint - INP). Asynchronous operations require immediate feedback: implement *Optimistic UI* when safe, displaying coherent loading *skeletons* or *spinners* without delaying action feedback.

### 3.3. Code Standards and DX (Developer Experience)

*   **Supreme Strict Mode:** The TypeScript compiler will operate with the `"strict": true` directive across the entire project scope. The use of the escape-hatch `any` type, undocumented `@ts-ignore` annotations, and implicit return types is **expressly prohibited** and will block Code Review approvals.
*   **Idempotency and Pure Functions:** State logic, data transformers, and utility functions must be pure and deterministic. They must consistently produce the same output for the same input, strictly isolating any unwanted _side-effects_.
*   **Sensible Performance and Optimization:** *Code Splitting* is mandatory at major route boundaries (using `React.lazy` and Suspense). Preemptive optimizations via `React.memo`, `useMemo`, or `useCallback` should not be adopted blindly. Use them surgically **only** when observation and profiling indicate actual re-rendering bottlenecks.

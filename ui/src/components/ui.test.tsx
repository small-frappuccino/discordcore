import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import {
  AdvancedTextInput,
  DashboardMetaList,
  DashboardPageSurface,
  EntityMultiPickerField,
  FeatureWorkspaceLayout,
  FlatPageLayout,
  KeyValueList,
  SettingsSelectField,
  UnsavedChangesBar,
} from "./ui";

describe("DashboardPageSurface", () => {
  it("renders notices above the shared page content", () => {
    render(
      <DashboardPageSurface
        notice={{ tone: "info", message: "Loading dashboard data." }}
        busyLabel="Refreshing..."
      >
        <div>Page content</div>
      </DashboardPageSurface>,
    );

    expect(screen.getByText("Loading dashboard data.")).toBeInTheDocument();
    expect(screen.getByText("Refreshing...")).toBeInTheDocument();
    expect(screen.queryByText("Workspace status")).not.toBeInTheDocument();
    expect(screen.getByText("Page content")).toBeInTheDocument();
  });
});

describe("FeatureWorkspaceLayout", () => {
  it("renders the shared feature workspace sections", () => {
    render(
      <FeatureWorkspaceLayout
        notice={{ tone: "error", message: "Workspace failed to load." }}
        busyLabel="Refreshing workspace..."
        summary={
          <section aria-label="Feature summary">
            <span>Summary strip</span>
          </section>
        }
        workspaceTitle="Manage widgets"
        workspaceDescription="Keep widget configuration focused and predictable."
        workspaceMeta={<span>2 configured</span>}
        workspaceContent={<div>Workspace body</div>}
        aside={
          <aside className="page-aside">
            <div>Aside body</div>
          </aside>
        }
      />,
    );

    expect(screen.getByText("Workspace failed to load.")).toBeInTheDocument();
    expect(screen.getByText("Refreshing workspace...")).toBeInTheDocument();
    expect(screen.queryByText("Workspace status")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Feature summary")).toHaveTextContent("Summary strip");
    expect(
      screen.getByRole("heading", { name: "Manage widgets", level: 2 }),
    ).toBeInTheDocument();
    expect(screen.getByText("2 configured")).toBeInTheDocument();
    expect(screen.getByText("Workspace body")).toBeInTheDocument();
    expect(screen.getByText("Aside body")).toBeInTheDocument();
    expect(document.querySelector(".feature-category-panel")).not.toBeNull();
    expect(document.querySelector(".content-grid-with-aside")).not.toBeNull();
  });

  it("falls back to a single-column workspace when no aside is provided", () => {
    const { container } = render(
      <FeatureWorkspaceLayout
        workspaceTitle="Single column"
        workspaceDescription="No secondary context is needed here."
        workspaceContent={<div>Only workspace content</div>}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Single column", level: 2 }),
    ).toBeInTheDocument();
    expect(screen.getByText("Only workspace content")).toBeInTheDocument();
    expect(container.querySelector(".content-grid-with-aside")).toBeNull();
  });
});

describe("FlatPageLayout", () => {
  it("applies the reusable flat shell classes on top of the shared workspace layout", () => {
    const { container } = render(
      <FlatPageLayout
        workspaceEyebrow={null}
        workspaceTitle={null}
        workspaceDescription={null}
      >
        <div>Flat workspace body</div>
      </FlatPageLayout>,
    );

    expect(screen.getByText("Flat workspace body")).toBeInTheDocument();
    expect(container.querySelector(".flat-page-surface")).not.toBeNull();
    expect(container.querySelector(".flat-page-layout")).not.toBeNull();
    expect(container.querySelector(".flat-page-workspace")).not.toBeNull();
    expect(container.querySelector(".feature-category-panel")).not.toBeNull();
  });
});

describe("DashboardMetaList", () => {
  it("suppresses shell-level context that should stay out of page bodies", () => {
    render(
      <DashboardMetaList
        items={[
          { label: "Server", value: "Operations" },
          { label: "Origin", value: "https://control.example.test" },
        ]}
      />,
    );

    expect(screen.queryByText(/Server:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Origin:/)).not.toBeInTheDocument();
  });
});

describe("KeyValueList", () => {
  it("filters internal source and override metadata in standard dashboard mode", () => {
    render(
      <KeyValueList
        items={[
          { label: "Applied from", value: "Inherited from global dashboard settings." },
          { label: "Override", value: "Configured here" },
          { label: "Current signal", value: "Choose a destination channel." },
        ]}
      />,
    );

    expect(screen.queryByText("Applied from")).not.toBeInTheDocument();
    expect(screen.queryByText("Override")).not.toBeInTheDocument();
    expect(screen.getByText("Current signal")).toBeInTheDocument();
    expect(screen.getByText("Choose a destination channel.")).toBeInTheDocument();
  });
});

describe("UnsavedChangesBar", () => {
  it("renders reset and save actions only when the page is dirty", async () => {
    const user = userEvent.setup();
    const onReset = vi.fn();
    const onSave = vi.fn();
    const { rerender } = render(
      <UnsavedChangesBar
        hasUnsavedChanges={false}
        onReset={onReset}
        onSave={onSave}
      />,
    );

    expect(screen.queryByText("Careful - you have unsaved changes.")).not.toBeInTheDocument();

    rerender(
      <UnsavedChangesBar
        hasUnsavedChanges
        onReset={onReset}
        onSave={onSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Reset" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(screen.getByText("Careful - you have unsaved changes.")).toBeInTheDocument();
    expect(onReset).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledTimes(1);
  });
});

describe("AdvancedTextInput", () => {
  it("keeps raw ID fallback controls out of the standard UI", () => {
    window.history.replaceState({}, "", "/manage/guild-1/core/commands");

    render(
      <AdvancedTextInput
        label="Channel ID fallback"
        inputLabel="Command channel ID fallback"
        value=""
        onChange={() => {}}
        placeholder="Discord channel ID"
      />,
    );

    expect(screen.queryByText("Advanced", { selector: "summary" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Command channel ID fallback")).not.toBeInTheDocument();
  });

  it("exposes diagnostic controls only in explicit diagnostic mode", () => {
    window.history.replaceState(
      {},
      "",
      "/manage/guild-1/core/commands?diagnostics=1",
    );

    render(
      <AdvancedTextInput
        label="Channel ID fallback"
        inputLabel="Command channel ID fallback"
        value=""
        onChange={() => {}}
        placeholder="Discord channel ID"
      />,
    );

    expect(screen.getByText("Advanced", { selector: "summary" })).toBeInTheDocument();
    expect(screen.getByLabelText("Command channel ID fallback")).toBeInTheDocument();

    window.history.replaceState({}, "", "/");
  });
});

describe("EntityMultiPickerField", () => {
  it("keeps the option list collapsed until the user opens it", async () => {
    const user = userEvent.setup();

    render(<EntityMultiPickerFieldHarness />);

    expect(screen.queryByRole("checkbox", { name: "Admin" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /staff roles/i })).toHaveTextContent(
      "Select one or more",
    );

    await user.click(screen.getByRole("button", { name: /staff roles/i }));

    expect(screen.getByRole("checkbox", { name: "Admin" })).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Moderator" })).toBeInTheDocument();

    await user.click(screen.getByRole("checkbox", { name: "Admin" }));

    expect(screen.getByRole("button", { name: /staff roles/i })).toHaveTextContent("Admin");
  });
});

describe("SettingsSelectField", () => {
  it("shows the current selection inline and expands a list when opened", async () => {
    const user = userEvent.setup();

    render(<SettingsSelectFieldHarness />);

    const trigger = screen.getByRole("button", { name: /mute role/i });
    expect(trigger).toHaveTextContent("No mute role");
    expect(screen.queryByRole("option", { name: "Muted" })).not.toBeInTheDocument();

    await user.click(trigger);

    expect(screen.getByRole("option", { name: "Muted" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Muted alt" })).toBeInTheDocument();

    await user.click(screen.getByRole("option", { name: "Muted" }));

    expect(screen.getByRole("button", { name: /mute role/i })).toHaveTextContent(
      "Muted",
    );
    expect(screen.queryByRole("option", { name: "Muted" })).not.toBeInTheDocument();
  });
});

function EntityMultiPickerFieldHarness() {
  const [selectedValues, setSelectedValues] = useState<string[]>([]);

  return (
    <EntityMultiPickerField
      label="Staff roles"
      options={[
        { value: "admin", label: "Admin" },
        { value: "moderator", label: "Moderator" },
      ]}
      selectedValues={selectedValues}
      onToggle={(value) =>
        setSelectedValues((current) =>
          current.includes(value)
            ? current.filter((entry) => entry !== value)
            : [...current, value],
        )
      }
    />
  );
}

function SettingsSelectFieldHarness() {
  const [value, setValue] = useState("");

  return (
    <SettingsSelectField
      label="Mute role"
      value={value}
      onChange={setValue}
      placeholder="No mute role"
      options={[
        { value: "muted", label: "Muted" },
        { value: "muted-alt", label: "Muted alt" },
      ]}
    />
  );
}

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import {
  DashboardPageSurface,
  EntityMultiPickerField,
  FeatureWorkspaceLayout,
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

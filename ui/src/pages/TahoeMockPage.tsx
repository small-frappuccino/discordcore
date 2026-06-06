import { useState } from "react";
import { PageContainer, PageHeader } from "../components/ui";
import {
  SettingsGroup,
  SettingsRow,
  ToggleSwitch,
  SelectMenu,
  ActionTrigger,
  SliderControl,
  SegmentedControl,
  VisualRadioGroup,
} from "../components/ui/tahoe";
import { Stack } from "../components/layout";

export function TahoeMockPage() {
  const [siriEnabled, setSiriEnabled] = useState(true);
  const [language, setLanguage] = useState("en-us");
  const [trackingSpeed, setTrackingSpeed] = useState(50);
  const [clickType, setClickType] = useState("point-click");
  const [appearance, setAppearance] = useState("auto");

  return (
    <PageContainer>
      <Stack spacing="xl">
        <PageHeader>
          <PageHeader.TitleRow>
            <PageHeader.Title>System Settings (Tahoe Mock)</PageHeader.Title>
          </PageHeader.TitleRow>
          <PageHeader.Description>A demonstration of the new Tahoe SOTA primitives.</PageHeader.Description>
        </PageHeader>

        <div className="max-w-3xl">
          <h2 className="text-sm font-semibold text-muted mb-2 ml-1">Siri Requests</h2>
          <SettingsGroup>
            <SettingsRow
              title="Siri"
              control={<ToggleSwitch checked={siriEnabled} onCheckedChange={setSiriEnabled} />}
            />
            <SettingsRow
              title="Listen for"
              control={<SelectMenu options={[{ value: "off", label: "Off" }, { value: "hey-siri", label: "\"Hey Siri\"" }]} value="off" />}
            />
            <SettingsRow
              title="Keyboard shortcut"
              description="Press to type to Siri"
              isMultiline
              control={<SelectMenu options={[{ value: "cmd-twice", label: "Press Either Command Key Twice" }]} value="cmd-twice" />}
            />
            <SettingsRow
              title="Language"
              control={
                <SelectMenu
                  options={[
                    { value: "en-us", label: "English (United States)" },
                    { value: "es-es", label: "Spanish (Spain)" },
                  ]}
                  value={language}
                  onChange={setLanguage}
                />
              }
            />
            <SettingsRow
              title="Siri history"
              control={<ActionTrigger>Delete Siri & Dictation History...</ActionTrigger>}
            />
          </SettingsGroup>

          <h2 className="text-sm font-semibold text-muted mb-2 mt-8 ml-1">Trackpad</h2>
          <SettingsGroup>
            <SettingsRow
              title="Tracking speed"
              control={<SliderControl value={trackingSpeed} onChange={setTrackingSpeed} minLabel="Slow" maxLabel="Fast" />}
            />
            <SettingsRow
              title="Interaction Mode"
              control={
                <SegmentedControl
                  options={[
                    { value: "point-click", label: "Point & Click" },
                    { value: "scroll-zoom", label: "Scroll & Zoom" },
                  ]}
                  value={clickType}
                  onChange={setClickType}
                />
              }
            />
          </SettingsGroup>

          <h2 className="text-sm font-semibold text-muted mb-2 mt-8 ml-1">Appearance</h2>
          <SettingsGroup>
            <SettingsRow
              title="System Appearance"
              control={
                <VisualRadioGroup
                  options={[
                    { value: "light", label: "Light", renderVisual: <div className="w-8 h-8 rounded bg-white" /> },
                    { value: "dark", label: "Dark", renderVisual: <div className="w-8 h-8 rounded bg-zinc-900" /> },
                    { value: "auto", label: "Auto", renderVisual: <div className="w-8 h-8 rounded bg-gradient-to-r from-white to-zinc-900" /> },
                  ]}
                  value={appearance}
                  onChange={setAppearance}
                />
              }
            />
          </SettingsGroup>
        </div>
      </Stack>
    </PageContainer>
  );
}

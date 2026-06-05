const fs = require("fs");
const path = require("path");

function replaceRegexInFile(filePath, replacements) {
  let content = fs.readFileSync(filePath, "utf8");
  for (const { regex, replace } of replacements) {
    content = content.replace(regex, replace);
  }
  fs.writeFileSync(filePath, content);
}

// 1. Box.tsx
replaceRegexInFile(path.join(__dirname, "ui/src/components/layout/Box.tsx"), [
  { regex: /import \{ ResponsiveProp, SpacingToken \} from "\.\.\/\.\.\/types\/layout";/g, replace: 'import type { ResponsiveProp, SpacingToken } from "../../types/layout";' }
]);

// 2. Cluster.tsx
replaceRegexInFile(path.join(__dirname, "ui/src/components/layout/Cluster.tsx"), [
  { regex: /import \{ BoxProps, ResponsiveProp, SpacingToken \} from "\.\/Box";/g, replace: 'import type { BoxProps, ResponsiveProp, SpacingToken } from "./Box";' }
]);

// 3. Grid.tsx
replaceRegexInFile(path.join(__dirname, "ui/src/components/layout/Grid.tsx"), [
  { regex: /import \{ BoxProps, ResponsiveProp, SpacingToken \} from "\.\/Box";/g, replace: 'import type { BoxProps, ResponsiveProp, SpacingToken } from "./Box";' }
]);

// 4. Stack.tsx
replaceRegexInFile(path.join(__dirname, "ui/src/components/layout/Stack.tsx"), [
  { regex: /import \{ BoxProps, ResponsiveProp, SpacingToken \} from "\.\/Box";/g, replace: 'import type { BoxProps, ResponsiveProp, SpacingToken } from "./Box";' }
]);

// 5. PageHeader.tsx
replaceRegexInFile(path.join(__dirname, "ui/src/components/ui/PageHeader/PageHeader.tsx"), [
  { regex: /import \{ StackProps \} from "\.\.\/\.\.\/layout";/g, replace: 'import type { StackProps } from "../../layout";' },
  { regex: /import \{ BoxProps \} from "\.\.\/\.\.\/layout\/Box";/g, replace: 'import type { BoxProps } from "../../layout/Box";' },
  { regex: /import \{ ClusterProps \} from "\.\.\/\.\.\/layout";/g, replace: 'import type { ClusterProps } from "../../layout";' }
]);

// 6. ModerationPage.tsx
replaceRegexInFile(path.join(__dirname, "ui/src/pages/ModerationPage.tsx"), [
  { regex: /import \{ Stack, Box \} from "\.\.\/components\/layout";/g, replace: 'import { Stack } from "../components/layout";' }
]);

console.log("Done");

const fs = require("fs");
const path = require("path");

function replaceInFile(filePath, search, replace) {
  let content = fs.readFileSync(filePath, "utf8");
  content = content.replace(search, replace);
  fs.writeFileSync(filePath, content);
}

// 1. Box.tsx
replaceInFile(
  path.join(__dirname, "ui/src/components/layout/Box.tsx"),
  'import { ResponsiveProp, SpacingToken, resolveSpacing } from "../../lib/layout-utils";',
  'import type { ResponsiveProp, SpacingToken } from "../../lib/layout-utils";\nimport { resolveSpacing } from "../../lib/layout-utils";'
);

// 2. Cluster.tsx
replaceInFile(
  path.join(__dirname, "ui/src/components/layout/Cluster.tsx"),
  'import { Box, BoxProps } from "./Box";\nimport { ResponsiveProp, SpacingToken, resolveSpacing } from "../../lib/layout-utils";',
  'import { Box } from "./Box";\nimport type { BoxProps } from "./Box";\nimport type { ResponsiveProp, SpacingToken } from "../../lib/layout-utils";\nimport { resolveSpacing } from "../../lib/layout-utils";'
);

// 3. Grid.tsx
replaceInFile(
  path.join(__dirname, "ui/src/components/layout/Grid.tsx"),
  'import { Box, BoxProps } from "./Box";\nimport { ResponsiveProp, SpacingToken, resolveSpacing } from "../../lib/layout-utils";',
  'import { Box } from "./Box";\nimport type { BoxProps } from "./Box";\nimport type { ResponsiveProp, SpacingToken } from "../../lib/layout-utils";\nimport { resolveSpacing } from "../../lib/layout-utils";'
);

// 4. Stack.tsx
replaceInFile(
  path.join(__dirname, "ui/src/components/layout/Stack.tsx"),
  'import { Box, BoxProps } from "./Box";\nimport { ResponsiveProp, SpacingToken, resolveSpacing } from "../../lib/layout-utils";',
  'import { Box } from "./Box";\nimport type { BoxProps } from "./Box";\nimport type { ResponsiveProp, SpacingToken } from "../../lib/layout-utils";\nimport { resolveSpacing } from "../../lib/layout-utils";'
);

// 5. PageHeader.tsx
replaceInFile(
  path.join(__dirname, "ui/src/components/ui/PageHeader/PageHeader.tsx"),
  'import { Stack, StackProps } from "../../layout/Stack";\nimport { Box, BoxProps } from "../../layout/Box";\nimport { Cluster, ClusterProps } from "../../layout/Cluster";',
  'import { Stack } from "../../layout/Stack";\nimport type { StackProps } from "../../layout/Stack";\nimport { Box } from "../../layout/Box";\nimport type { BoxProps } from "../../layout/Box";\nimport { Cluster } from "../../layout/Cluster";\nimport type { ClusterProps } from "../../layout/Cluster";'
);

// 6. ModerationPage.tsx
replaceInFile(
  path.join(__dirname, "ui/src/pages/ModerationPage.tsx"),
  'import { Stack, Box } from "../components/layout";',
  'import { Stack } from "../components/layout";'
);

console.log("Done");

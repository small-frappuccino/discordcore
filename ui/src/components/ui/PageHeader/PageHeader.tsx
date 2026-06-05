import { cn } from "../../../lib/utils";
import { Slot } from "../Slot/Slot";
import { Stack } from "../../layout/Stack";
import type { StackProps } from "../../layout/Stack";
import { Box } from "../../layout/Box";
import type { BoxProps } from "../../layout/Box";
import { Cluster } from "../../layout/Cluster";
import type { ClusterProps } from "../../layout/Cluster";

export interface PageHeaderProps extends StackProps {
  asChild?: boolean;
}

function PageHeaderRoot({ className, asChild, children, spacing = "sm", ...props }: PageHeaderProps) {
  if (asChild) {
    return (
      <Slot className={cn("page-header mb-8", className)} {...props}>
        {children}
      </Slot>
    );
  }
  return (
    <Stack className={cn("page-header mb-8", className)} spacing={spacing} {...props}>
      {children}
    </Stack>
  );
}

export interface PageHeaderTitleProps extends BoxProps {
  asChild?: boolean;
}

function PageHeaderTitle({ className, asChild, children, ...props }: PageHeaderTitleProps) {
  if (asChild) {
    return (
      <Slot className={cn("page-title text-3xl font-bold tracking-tight text-text-primary", className)} {...props}>
        {children}
      </Slot>
    );
  }
  return (
    <Box as="h1" className={cn("page-title text-3xl font-bold tracking-tight text-text-primary", className)} {...props}>
      {children}
    </Box>
  );
}

export interface PageHeaderDescriptionProps extends BoxProps {
  asChild?: boolean;
}

function PageHeaderDescription({ className, asChild, children, ...props }: PageHeaderDescriptionProps) {
  if (asChild) {
    return (
      <Slot className={cn("page-description text-lg text-text-secondary max-w-[700px]", className)} {...props}>
        {children}
      </Slot>
    );
  }
  return (
    <Box as="p" className={cn("page-description text-lg text-text-secondary max-w-[700px]", className)} {...props}>
      {children}
    </Box>
  );
}

export interface PageHeaderTitleRowProps extends ClusterProps {
  asChild?: boolean;
}

function PageHeaderTitleRow({ className, asChild, children, spacing = "md", align = "center", ...props }: PageHeaderTitleRowProps) {
  if (asChild) {
    return (
      <Slot className={cn("page-header-title-row", className)} {...props}>
        {children}
      </Slot>
    );
  }
  return (
    <Cluster className={cn("page-header-title-row", className)} spacing={spacing} align={align} {...props}>
      {children}
    </Cluster>
  );
}

export const PageHeader = Object.assign(PageHeaderRoot, {
  TitleRow: PageHeaderTitleRow,
  Title: PageHeaderTitle,
  Description: PageHeaderDescription,
});

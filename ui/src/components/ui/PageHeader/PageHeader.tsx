import * as React from "react";

type PageHeaderProps = {
  title: string;
  description?: string;
  badge?: React.ReactNode;
};

export function PageHeader({ title, description, badge }: PageHeaderProps) {
  return (
    <div className="page-header">
      <div className="page-header-title-row">
        <h1 className="page-title">{title}</h1>
        {badge}
      </div>
      {description && <p className="page-description">{description}</p>}
    </div>
  );
}

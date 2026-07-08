import type { ReactNode } from "react";
import {
  FieldDescription,
  FieldGroup,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field";

export function SettingBlock({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <FieldSet className="border-b border-border pb-5 last:border-b-0 last:pb-0">
      <FieldLegend variant="label">{title}</FieldLegend>
      <FieldDescription>{description}</FieldDescription>
      <FieldGroup>{children}</FieldGroup>
    </FieldSet>
  );
}

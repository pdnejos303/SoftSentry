import * as React from "react";
import { cn } from "@/lib/utils";

/**
 * A connected cluster of related actions rendered as ONE clean control: a single
 * bordered surface with hairline dividers between items. Children should be
 * `<Button variant="ghost">` (or a component that renders one) so they merge
 * seamlessly into a calm toolbar instead of competing as separate pills.
 *
 * Focus rings are forced inset so `overflow-hidden` (which clips the outer
 * corners square) doesn't swallow them.
 */
export function ButtonGroup({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "inline-flex divide-x divide-border overflow-hidden rounded-md border bg-card",
        "[&>button]:rounded-none [&>button]:border-0",
        "[&>button:focus-visible]:ring-inset [&>button:focus-visible]:ring-offset-0",
        className,
      )}
      {...props}
    />
  );
}

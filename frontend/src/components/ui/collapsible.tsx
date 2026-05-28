"use client"

import { Collapsible as CollapsiblePrimitive } from "@base-ui/react/collapsible"

function Collapsible({ ...props }: CollapsiblePrimitive.Root.Props) {
  return <CollapsiblePrimitive.Root data-slot="collapsible" {...props} />
}

export { Collapsible }
export { CollapsibleTrigger } from "@/components/ui/collapsible-trigger"
export { CollapsibleContent } from "@/components/ui/collapsible-content"

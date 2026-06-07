"use client"

import * as React from "react"
import { Switch as SwitchPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

function Switch({
  className,
  size = "default",
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root> & {
  size?: "sm" | "default"
}) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      data-size={size}
      className={cn(
        // iOS-style switch: rounded-full pill shape
        "peer group/switch relative inline-flex shrink-0 cursor-pointer items-center rounded-full border-0 transition-all duration-200 outline-none after:absolute after:-inset-x-3 after:-inset-y-2 focus-visible:ring-3 focus-visible:ring-ring/50",
        // Sizing — iOS proportions (51×31 default, 41×25 small)
        "data-[size=default]:h-[31px] data-[size=default]:w-[51px]",
        "data-[size=sm]:h-[25px] data-[size=sm]:w-[41px]",
        // Colors: blue when on, gray when off
        "data-checked:bg-blue-500 dark:data-checked:bg-blue-500",
        "data-unchecked:bg-zinc-300 dark:data-unchecked:bg-zinc-600",
        // Disabled state
        "data-disabled:cursor-not-allowed data-disabled:opacity-50",
        className
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className={cn(
          // iOS white thumb with subtle shadow
          "pointer-events-none block rounded-full bg-white shadow-md ring-0 transition-transform duration-200",
          // Sizing: slightly smaller than track height
          "group-data-[size=default]/switch:size-[27px]",
          "group-data-[size=sm]/switch:size-[21px]",
          // Position: 2px inset from track edge
          "group-data-[size=default]/switch:data-unchecked:translate-x-[2px]",
          "group-data-[size=default]/switch:data-checked:translate-x-[22px]",
          "group-data-[size=sm]/switch:data-unchecked:translate-x-[2px]",
          "group-data-[size=sm]/switch:data-checked:translate-x-[18px]"
        )}
      />
    </SwitchPrimitive.Root>
  )
}

export { Switch }

import * as React from 'react'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  /** 无障碍标签（必须提供） */
  label: string
  /** 是否显示 tooltip，默认 true */
  tooltip?: boolean
}

/**
 * 带 aria-label 和可选 tooltip 的图标按钮。
 * 所有 icon-only 按钮都应使用此组件，确保可访问性。
 */
export const IconButton = React.forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ label, tooltip = true, className, children, ...props }, ref) => {
    const button = (
      <button
        ref={ref}
        type="button"
        aria-label={label}
        className={cn(
          'inline-flex items-center justify-center rounded p-1.5 transition-colors',
          'hover:bg-[var(--muted)] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--ring)]',
          'disabled:pointer-events-none disabled:opacity-50',
          className,
        )}
        {...props}
      >
        {children}
      </button>
    )

    if (!tooltip) return button

    return (
      <Tooltip>
        <TooltipTrigger asChild>{button}</TooltipTrigger>
        <TooltipContent>{label}</TooltipContent>
      </Tooltip>
    )
  },
)
IconButton.displayName = 'IconButton'

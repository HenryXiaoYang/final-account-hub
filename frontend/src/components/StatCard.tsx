import { cn } from '@/lib/utils'

interface StatBarProps {
  items: { label: string; value: number | string; color?: string }[]
  className?: string
}

export function StatBar({ items, className }: StatBarProps) {
  return (
    <div className={cn('flex flex-wrap items-center gap-x-4 gap-y-1 text-sm', className)}>
      {items.map((item) => (
        <div key={item.label} className="flex items-center gap-1.5">
          <span className="text-[var(--muted-foreground)]">{item.label}</span>
          <span className={cn('font-semibold tabular-nums', item.color)}>{item.value}</span>
        </div>
      ))}
    </div>
  )
}

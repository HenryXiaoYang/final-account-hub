import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer,
} from 'recharts'

export interface ChartRow {
  date: string
  [key: string]: string | number
}

interface TrendChartProps {
  data: ChartRow[]
  isDark: boolean
  label: string
  dataKey?: string
  color?: string
}

export default function TrendChart({ data, isDark, label, dataKey = 'Available', color = '#3b82f6' }: TrendChartProps) {
  const gridColor = isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.05)'
  const textColor = isDark ? '#838b9e' : '#5c6578'
  const gradientId = `fill-${dataKey}`

  return (
    <ResponsiveContainer width="100%" height="100%">
      <AreaChart data={data}>
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor={color} stopOpacity={0.2} />
            <stop offset="95%" stopColor={color} stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
        <XAxis dataKey="date" tick={{ fill: textColor, fontSize: 11 }} />
        <YAxis tick={{ fill: textColor, fontSize: 11 }} allowDecimals={false} />
        <RechartsTooltip
          contentStyle={{
            background: isDark ? '#16181f' : '#fff',
            border: `1px solid ${isDark ? '#262a36' : '#e2e5eb'}`,
            borderRadius: 8,
            fontSize: 12,
          }}
        />
        <Area
          type="monotone"
          dataKey={dataKey}
          name={label}
          stroke={color}
          strokeWidth={2}
          fill={`url(#${gradientId})`}
          dot={false}
        />
      </AreaChart>
    </ResponsiveContainer>
  )
}

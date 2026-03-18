import { Component, type ReactNode } from 'react'
import { RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import i18n from '@/i18n'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  handleRetry = () => {
    this.setState({ error: null })
  }

  render() {
    if (this.state.error) {
      if (this.props.fallback) return this.props.fallback

      return (
        <div className="flex flex-col items-center justify-center gap-3 py-16 text-center" role="alert">
          <p className="text-sm font-medium text-[var(--foreground)]">{i18n.t('common.somethingWentWrong')}</p>
          <p className="text-xs text-[var(--muted-foreground)] max-w-md break-words">
            {this.state.error.message}
          </p>
          <Button variant="outline" size="sm" onClick={this.handleRetry}>
            <RefreshCw className="h-3 w-3" />
            {i18n.t('common.retry')}
          </Button>
        </div>
      )
    }

    return this.props.children
  }
}

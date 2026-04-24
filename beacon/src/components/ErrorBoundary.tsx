import { Component, type ReactNode } from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  render() {
    if (this.state.error) {
      return (
        <div className="min-h-screen bg-[#0d1117] flex items-center justify-center p-4">
          <div className="max-w-md w-full bg-[#161b22] border border-[#f8514930] rounded-xl p-6">
            <div className="flex items-center gap-3 mb-3">
              <AlertTriangle size={18} className="text-[#f85149]" />
              <h1 className="text-[#e6edf3] font-semibold text-sm">Something went wrong</h1>
            </div>
            <p className="text-[#8b949e] text-xs mb-4 leading-relaxed">
              An unexpected error occurred in the application. Refreshing the page usually resolves this.
            </p>
            <pre className="text-[10px] text-[#484f58] bg-[#0d1117] rounded p-3 overflow-x-auto max-h-32 font-mono mb-4">
              {this.state.error.message}
            </pre>
            <button
              onClick={() => window.location.reload()}
              className="flex items-center justify-center gap-2 w-full px-4 py-2 bg-[#21262d] text-[#e6edf3] text-sm rounded-lg hover:bg-[#30363d] transition-colors"
            >
              <RefreshCw size={13} />
              Reload page
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

import { Component, type ReactNode } from 'react';
import { Button, Result } from 'antd';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * Last-resort render guard: a rendering error in one page must not
 * white-screen the whole console.
 */
export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: unknown) {
    console.error('[ErrorBoundary]', error, info);
  }

  render() {
    if (this.state.error) {
      return (
        <div style={{ minHeight: '60vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Result
            status="error"
            title="Something went wrong"
            subTitle={
              <span style={{ fontFamily: 'SF Mono, Menlo, monospace', fontSize: 12 }}>{this.state.error.message}</span>
            }
            extra={
              <Button type="primary" onClick={() => window.location.reload()}>
                Reload
              </Button>
            }
          />
        </div>
      );
    }
    return this.props.children;
  }
}

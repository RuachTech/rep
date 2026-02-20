import { useState } from 'react';
import { useRep } from '@rep-protocol/react';
import { TodoList } from './components/TodoList';
import { RepConfigPanel } from './components/RepConfigPanel';
import type { Todo } from './types';

// Env badge colours keyed by ENV_NAME value.
const ENV_COLOURS: Record<string, string> = {
  production: '#ef4444',
  staging:    '#f59e0b',
  development:'#22c55e',
};

export default function App() {
  // useRep() reads REP_PUBLIC_* vars injected by the gateway — synchronous, no
  // loading state. Falls back to the second arg when the gateway is not running.
  const appTitle  = useRep('APP_TITLE',  'REP Todo');
  const envName   = useRep('ENV_NAME',   'development');
  const maxTodosStr = useRep('MAX_TODOS', '10');
  const maxTodos  = parseInt(maxTodosStr ?? '10', 10);

  const [todos, setTodos] = useState<Todo[]>([
    { id: '1', text: 'Read the REP spec',   completed: true,  createdAt: new Date() },
    { id: '2', text: 'Run the gateway',     completed: false, createdAt: new Date() },
    { id: '3', text: 'Ship to production',  completed: false, createdAt: new Date() },
  ]);
  const [showConfig, setShowConfig] = useState(false);

  function addTodo(text: string) {
    if (todos.length >= maxTodos) return;
    setTodos(prev => [
      ...prev,
      { id: crypto.randomUUID(), text, completed: false, createdAt: new Date() },
    ]);
  }

  function toggleTodo(id: string) {
    setTodos(prev => prev.map(t => t.id === id ? { ...t, completed: !t.completed } : t));
  }

  function deleteTodo(id: string) {
    setTodos(prev => prev.filter(t => t.id !== id));
  }

  const badgeColour = ENV_COLOURS[envName ?? 'development'] ?? '#6b7280';

  return (
    <div style={{ maxWidth: 640, margin: '0 auto', padding: '2rem 1rem', fontFamily: 'system-ui, -apple-system, sans-serif' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '1.5rem' }}>
        <div>
          {/* appTitle comes from REP_PUBLIC_APP_TITLE — re-renders on hot reload */}
          <h1 style={{ margin: 0, fontSize: '1.75rem', fontWeight: 700, color: '#111827' }}>
            {appTitle}
          </h1>
          {/* envName comes from REP_PUBLIC_ENV_NAME */}
          <span style={{
            display: 'inline-block', marginTop: '0.35rem',
            padding: '0.15rem 0.55rem', borderRadius: '0.25rem',
            background: badgeColour, color: '#fff',
            fontSize: '0.7rem', fontWeight: 700,
            textTransform: 'uppercase', letterSpacing: '0.06em',
          }}>
            {envName}
          </span>
        </div>
        <button
          onClick={() => setShowConfig(v => !v)}
          style={{
            marginTop: '0.25rem', padding: '0.45rem 0.9rem',
            borderRadius: '0.375rem', border: '1px solid #d1d5db',
            background: showConfig ? '#eff6ff' : '#f9fafb',
            color: showConfig ? '#2563eb' : '#374151',
            cursor: 'pointer', fontSize: '0.875rem', fontWeight: 500,
          }}
        >
          {showConfig ? '▲ Hide Config' : '▼ Show Config'}
        </button>
      </header>

      {/* Config panel shows all gateway-injected vars (public + sensitive) */}
      {showConfig && <RepConfigPanel />}

      {/* MAX_TODOS from REP_PUBLIC_MAX_TODOS controls the list limit */}
      <TodoList
        todos={todos}
        maxTodos={maxTodos}
        onAdd={addTodo}
        onToggle={toggleTodo}
        onDelete={deleteTodo}
      />

      <footer style={{ marginTop: '2rem', paddingTop: '1rem', borderTop: '1px solid #e5e7eb', fontSize: '0.75rem', color: '#9ca3af' }}>
        Powered by{' '}
        <a href="https://github.com/ruachtech/rep" style={{ color: '#6b7280' }}>REP Gateway</a>
        {' '}— runtime config injected at container start, not at build time.
      </footer>
    </div>
  );
}

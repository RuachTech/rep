import { useState } from 'react';
import { TodoItem } from './TodoItem';
import type { Todo } from '../types';

interface Props {
  todos: Todo[];
  /** Sourced from REP_PUBLIC_MAX_TODOS — changes at container runtime, not rebuild. */
  maxTodos: number;
  onAdd: (text: string) => void;
  onToggle: (id: string) => void;
  onDelete: (id: string) => void;
}

export function TodoList({ todos, maxTodos, onAdd, onToggle, onDelete }: Props) {
  const [input, setInput] = useState('');
  const atLimit = todos.length >= maxTodos;
  const remaining = todos.filter(t => !t.completed).length;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const text = input.trim();
    if (!text || atLimit) return;
    onAdd(text);
    setInput('');
  }

  return (
    <section>
      <form onSubmit={handleSubmit} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <input
          type="text"
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder={atLimit ? `Limit of ${maxTodos} reached (set by gateway)` : 'What needs doing?'}
          disabled={atLimit}
          style={{
            flex: 1, padding: '0.55rem 0.75rem', borderRadius: '0.375rem',
            border: '1px solid #d1d5db', fontSize: '1rem',
            color: '#111827', outline: 'none',
            opacity: atLimit ? 0.55 : 1,
          }}
        />
        <button
          type="submit"
          disabled={atLimit || !input.trim()}
          style={{
            padding: '0.55rem 1.25rem', borderRadius: '0.375rem',
            background: '#3b82f6', color: '#fff', border: 'none',
            fontSize: '1rem', fontWeight: 500, cursor: 'pointer',
            opacity: (atLimit || !input.trim()) ? 0.45 : 1,
          }}
        >
          Add
        </button>
      </form>

      {atLimit && (
        <p style={{
          margin: '0 0 1rem', padding: '0.5rem 0.75rem',
          background: '#fffbeb', border: '1px solid #fde68a',
          borderRadius: '0.375rem', fontSize: '0.8rem', color: '#92400e',
        }}>
          Todo limit ({maxTodos}) reached. This limit comes from{' '}
          <code>REP_PUBLIC_MAX_TODOS</code> — change it by restarting the gateway
          with a different value. No rebuild needed.
        </p>
      )}

      {todos.length === 0 ? (
        <p style={{ color: '#9ca3af', fontSize: '0.9rem', textAlign: 'center', padding: '2rem 0' }}>
          No todos yet. Add one above.
        </p>
      ) : (
        <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
          {todos.map(todo => (
            <TodoItem key={todo.id} todo={todo} onToggle={onToggle} onDelete={onDelete} />
          ))}
        </ul>
      )}

      {todos.length > 0 && (
        <p style={{ marginTop: '0.75rem', fontSize: '0.8rem', color: '#9ca3af' }}>
          {remaining} remaining · {todos.length}/{maxTodos} used
          · limit from <code style={{ fontSize: '0.75rem' }}>REP_PUBLIC_MAX_TODOS</code>
        </p>
      )}
    </section>
  );
}

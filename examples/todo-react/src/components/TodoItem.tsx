import type { Todo } from '../types';

interface Props {
  todo: Todo;
  onToggle: (id: string) => void;
  onDelete: (id: string) => void;
}

export function TodoItem({ todo, onToggle, onDelete }: Props) {
  return (
    <li style={{
      display: 'flex', alignItems: 'center', gap: '0.75rem',
      padding: '0.65rem 0.75rem', marginBottom: '0.5rem',
      background: todo.completed ? '#f9fafb' : '#fff',
      borderRadius: '0.375rem', border: '1px solid #e5e7eb',
      transition: 'background 0.15s',
    }}>
      <input
        type="checkbox"
        checked={todo.completed}
        onChange={() => onToggle(todo.id)}
        style={{ width: 17, height: 17, cursor: 'pointer', accentColor: '#3b82f6', flexShrink: 0 }}
      />
      <span style={{
        flex: 1, fontSize: '0.975rem',
        textDecoration: todo.completed ? 'line-through' : 'none',
        color: todo.completed ? '#9ca3af' : '#111827',
      }}>
        {todo.text}
      </span>
      <button
        onClick={() => onDelete(todo.id)}
        aria-label={`Delete "${todo.text}"`}
        style={{
          padding: '0.2rem 0.45rem', borderRadius: '0.25rem',
          border: 'none', background: 'transparent', cursor: 'pointer',
          color: '#d1d5db', fontSize: '1.1rem', lineHeight: 1,
        }}
        onMouseEnter={e => (e.currentTarget.style.color = '#ef4444')}
        onMouseLeave={e => (e.currentTarget.style.color = '#d1d5db')}
      >
        ×
      </button>
    </li>
  );
}

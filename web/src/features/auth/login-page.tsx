import { type FormEvent, useState } from 'react'
import { useLogin } from './use-login'

export function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const { mutate, isPending, error } = useLogin()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    mutate({ username, password })
  }

  return (
    <div
      style={{
        background: 'var(--bg-1)',
        border: '1px solid var(--border)',
        borderRadius: 'var(--r-card)',
        padding: 'var(--s-6)',
      }}
    >
      <h1
        style={{
          fontFamily: 'var(--font-ui)',
          fontSize: 22,
          fontWeight: 600,
          color: 'var(--fg-1)',
          margin: '0 0 var(--s-2)',
        }}
      >
        Sign in
      </h1>
      <p style={{ color: 'var(--fg-2)', fontSize: 14, margin: '0 0 var(--s-5)' }}>
        Enter your credentials to continue.
      </p>

      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--s-3)' }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 'var(--s-1)' }}>
          <span className="eyebrow">Username</span>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
            style={inputStyle}
          />
        </label>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 'var(--s-1)' }}>
          <span className="eyebrow">Password</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
            style={inputStyle}
          />
        </label>

        {error && (
          <p style={{ color: 'var(--danger)', fontSize: 13, margin: 0 }}>
            {error.message}
          </p>
        )}

        <button
          type="submit"
          disabled={isPending}
          style={{
            marginTop: 'var(--s-2)',
            height: 36,
            background: 'var(--ink-1)',
            color: 'var(--paper-0)',
            border: 'none',
            borderRadius: 'var(--r-input)',
            fontFamily: 'var(--font-ui)',
            fontSize: 14,
            fontWeight: 600,
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.7 : 1,
            transition: 'opacity var(--dur-micro) var(--ease)',
          }}
        >
          {isPending ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  height: 32,
  background: 'var(--bg)',
  border: '1px solid var(--border)',
  borderRadius: 'var(--r-input)',
  padding: '0 var(--s-3)',
  color: 'var(--fg-1)',
  fontFamily: 'var(--font-ui)',
  fontSize: 14,
  outline: 'none',
  transition: 'border-color var(--dur-micro) var(--ease)',
}

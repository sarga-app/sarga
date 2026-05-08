import { type FormEvent, useState } from 'react'
import { useLogin } from './use-login'

export function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const { mutate, isPending, error } = useLogin()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    mutate({ username, password })
  }

  return (
    <div
      className="min-h-screen flex flex-col items-center justify-center px-4 py-12 sm:px-6"
      style={{ background: 'var(--bg)' }}
    >
      {/* Brand */}
      <div className="mb-8 text-center select-none">
        <p
          className="sarga-bar"
          style={{
            fontFamily: 'var(--font-editorial)',
            fontSize: 'clamp(30px, 6vw, 40px)',
            fontWeight: 500,
            color: 'var(--fg-1)',
            letterSpacing: '-0.015em',
            margin: 0,
            lineHeight: 1,
          }}
        >
          sarga
        </p>
        <p
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 11,
            fontWeight: 500,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: 'var(--fg-3)',
            marginTop: 10,
          }}
        >
          Your music. Your server.
        </p>
      </div>

      {/* Card */}
      <div
        className="w-full"
        style={{
          maxWidth: 400,
          background: 'var(--bg-1)',
          border: '1px solid var(--border-strong)',
          borderRadius: 'var(--r-sheet)',
          padding: 'clamp(24px, 6vw, 40px)',
          boxShadow: 'var(--shadow-float)',
        }}
      >
        <h1
          style={{
            fontFamily: 'var(--font-ui)',
            fontSize: 18,
            fontWeight: 600,
            color: 'var(--fg-1)',
            margin: '0 0 var(--s-5)',
          }}
        >
          Sign in
        </h1>

        <form
          onSubmit={handleSubmit}
          style={{ display: 'flex', flexDirection: 'column', gap: 'var(--s-4)' }}
        >
          {/* Username */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--s-1)' }}>
            <label
              htmlFor="username"
              className="eyebrow"
              style={{ cursor: 'default' }}
            >
              Username
            </label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              required
              style={inputStyle}
            />
          </div>

          {/* Password */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--s-1)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
              <label
                htmlFor="password"
                className="eyebrow"
                style={{ cursor: 'default' }}
              >
                Password
              </label>
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                style={{
                  fontFamily: 'var(--font-ui)',
                  fontSize: 12,
                  color: 'var(--fg-3)',
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  padding: 0,
                }}
              >
                {showPassword ? 'Hide' : 'Show'}
              </button>
            </div>
            <input
              id="password"
              type={showPassword ? 'text' : 'password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
              style={inputStyle}
            />
          </div>

          {/* Error */}
          {error && (
            <p
              role="alert"
              style={{
                fontFamily: 'var(--font-ui)',
                fontSize: 13,
                color: 'var(--danger)',
                margin: 0,
                padding: 'var(--s-2) var(--s-3)',
                background: 'color-mix(in srgb, var(--danger) 8%, transparent)',
                borderRadius: 'var(--r-input)',
                border: '1px solid color-mix(in srgb, var(--danger) 20%, transparent)',
              }}
            >
              {error.message}
            </p>
          )}

          {/* Submit */}
          <button
            type="submit"
            disabled={isPending}
            style={{
              marginTop: 'var(--s-1)',
              height: 44,
              background: isPending ? 'var(--ink-2)' : 'var(--ink-1)',
              color: 'var(--paper-0)',
              border: 'none',
              borderRadius: 'var(--r-input)',
              fontFamily: 'var(--font-ui)',
              fontSize: 15,
              fontWeight: 600,
              cursor: isPending ? 'not-allowed' : 'pointer',
              transition: 'background var(--dur-micro) var(--ease)',
              letterSpacing: '-0.01em',
            }}
          >
            {isPending ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </div>

      {/* Footer hint */}
      <p
        style={{
          marginTop: 'var(--s-5)',
          fontFamily: 'var(--font-mono)',
          fontSize: 11,
          color: 'var(--fg-4)',
          textAlign: 'center',
        }}
      >
        Self-hosted · No account creation here
      </p>
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  height: 44,
  background: 'var(--bg)',
  border: '1px solid var(--border-strong)',
  borderRadius: 'var(--r-input)',
  padding: '0 var(--s-3)',
  color: 'var(--fg-1)',
  fontFamily: 'var(--font-ui)',
  fontSize: 15,
  outline: 'none',
  width: '100%',
  transition: 'border-color var(--dur-micro) var(--ease)',
}

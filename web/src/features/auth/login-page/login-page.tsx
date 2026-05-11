import { type FormEvent, useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { LogoWordmark } from '@/assets/LogoWordmark'
import { useLogin } from '../use-login'
import { cn } from '@/lib/cn'
import styles from './login-page.module.css'

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
    <div className={styles.page}>
      <div className={styles.brand}>
        <LogoWordmark height={52} />
      </div>

      <div className={styles.form}>
        <h1 className={styles.heading}>Sign in</h1>

        <form onSubmit={handleSubmit} className={styles.fields}>
          <div className={styles.field}>
            <label htmlFor="username" className={cn('eyebrow', styles.fieldLabel)}>
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
              className={styles.input}
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="password" className={cn('eyebrow', styles.fieldLabel)}>
              Password
            </label>
            <div className={styles.passwordWrap}>
              <input
                id="password"
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
                className={cn(styles.input, styles.inputPassword)}
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? 'Hide password' : 'Show password'}
                className={styles.eyeBtn}
              >
                {showPassword ? <EyeOff size={15} strokeWidth={1.5} /> : <Eye size={15} strokeWidth={1.5} />}
              </button>
            </div>
          </div>

          {error && (
            <p role="alert" className={styles.error}>{error.message}</p>
          )}

          <button type="submit" disabled={isPending} className={styles.submitBtn}>
            {isPending ? 'Signing in…' : 'Sign in'}
          </button>
        </form>

        <p className={styles.footer}>Self-hosted · No account creation here</p>
      </div>
    </div>
  )
}

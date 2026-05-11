import { useRef, useState, useEffect } from 'react'
import type { ImageFadeProps } from './image-fade.types'

export function ImageFade({ src, alt, style, placeholderStyle }: ImageFadeProps) {
  const imgRef = useRef<HTMLImageElement>(null)
  const [status, setStatus] = useState<'loading' | 'loaded' | 'error'>('loading')

  useEffect(() => {
    setStatus('loading')
    const img = imgRef.current
    if (!img) return
    if (img.complete) {
      setStatus(img.naturalWidth > 0 ? 'loaded' : 'error')
    }
  }, [src])

  return (
    <div style={{ position: 'relative', ...placeholderStyle }}>
      {status !== 'loaded' && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            background: 'var(--bg-3)',
            borderRadius: 'inherit',
          }}
        />
      )}
      <img
        ref={imgRef}
        src={src}
        alt={alt}
        loading="lazy"
        onLoad={() => setStatus('loaded')}
        onError={() => setStatus('error')}
        style={{
          display: 'block',
          opacity: status === 'loaded' ? 1 : 0,
          transition: status === 'loaded' ? 'opacity 200ms var(--ease)' : 'none',
          ...style,
        }}
      />
    </div>
  )
}

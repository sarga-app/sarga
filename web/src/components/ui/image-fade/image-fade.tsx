import type { ImageFadeProps } from './image-fade.types'

export function ImageFade({ src, alt, style, placeholderStyle }: ImageFadeProps) {
  return (
    <div style={{ background: 'var(--bg-3)', ...placeholderStyle }}>
      <img
        src={src}
        alt={alt}
        loading="lazy"
        decoding="async"
        style={{ display: 'block', ...style }}
      />
    </div>
  )
}

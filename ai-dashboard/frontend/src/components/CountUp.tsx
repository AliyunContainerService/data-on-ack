import { useEffect, useRef, useState } from 'react'

const prefersReducedMotion = () =>
  typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

/**
 * Animates a numeric value toward `value` with an ease-out curve.
 * Falls back to the raw value when reduced motion is preferred.
 */
export function useCountUp(value: number, duration = 900): number {
  const [display, setDisplay] = useState(value)
  const fromRef = useRef(0)
  const rafRef = useRef<number>(0)

  useEffect(() => {
    if (prefersReducedMotion()) {
      setDisplay(value)
      return
    }
    const from = fromRef.current
    const start = performance.now()
    const tick = (now: number) => {
      const t = Math.min((now - start) / duration, 1)
      // easeOutExpo
      const eased = t === 1 ? 1 : 1 - Math.pow(2, -10 * t)
      setDisplay(from + (value - from) * eased)
      if (t < 1) {
        rafRef.current = requestAnimationFrame(tick)
      } else {
        fromRef.current = value
      }
    }
    rafRef.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(rafRef.current)
  }, [value, duration])

  return display
}

interface CountUpProps {
  value: number
  decimals?: number
  duration?: number
  className?: string
  style?: React.CSSProperties
}

export default function CountUp({ value, decimals = 0, duration, className, style }: CountUpProps) {
  const display = useCountUp(value, duration)
  return (
    <span className={`tnum ${className || ''}`} style={style}>
      {display.toLocaleString(undefined, {
        minimumFractionDigits: decimals,
        maximumFractionDigits: decimals,
      })}
    </span>
  )
}

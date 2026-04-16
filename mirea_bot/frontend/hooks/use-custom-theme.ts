"use client"

import { useState, useEffect, useCallback } from "react"

const CUSTOM_HUE_KEY            = "custom_primary_hue"
const CUSTOM_BRIGHTNESS_KEY     = "custom_primary_brightness"
const CUSTOM_BG_HUE_KEY         = "custom_bg_hue"
const CUSTOM_BG_BRIGHTNESS_KEY  = "custom_bg_brightness"
const CUSTOM_BG_SATURATION_KEY  = "custom_bg_saturation"

export const COLOR_PRESETS = [
  { label: "Синий",    hue: 267 },
  { label: "Фиолет",  hue: 290 },
  { label: "Розовый", hue: 350 },
  { label: "Красный", hue: 27  },
  { label: "Оранж",   hue: 40  },
  { label: "Зелёный", hue: 165 },
  { label: "Циан",    hue: 200 },
]

export const BG_PRESETS = [
  { label: "Нейтральный", hue: 95  },
  { label: "Синий",       hue: 240 },
  { label: "Зелёный",     hue: 150 },
  { label: "Розовый",     hue: 350 },
  { label: "Оранж",       hue: 40  },
  { label: "Фиолет",      hue: 280 },
]

export const DEFAULT_HUE            = 267
export const DEFAULT_BRIGHTNESS     = 60   // 0–100, maps to 0.4–0.8 lightness
export const DEFAULT_BG_HUE         = 95
export const DEFAULT_BG_BRIGHTNESS  = 70  // 0–100, maps to 0.12–0.99 lightness
export const DEFAULT_BG_SATURATION  = 50  // 0–100, at 50 = current chroma, 0 = gray, 100 = 2x

// brightness: 0–100 → lightness 0.12–0.96 (чёрный → белый)
function brightnessToL(b: number) {
  return 0.12 + (b / 100) * 0.84
}

function applyPrimary(hue: number, brightness: number) {
  const L = brightnessToL(brightness)
  const Laccent = Math.min(L + 0.07, 0.85)
  const Lring   = Math.max(L - 0.08, 0.3)
  const root = document.documentElement
  root.style.setProperty("--primary",              `oklch(${L.toFixed(3)} 0.22 ${hue})`)
  root.style.setProperty("--primary-foreground",   "oklch(1 0 0)")
  root.style.setProperty("--accent",               `oklch(${Laccent.toFixed(3)} 0.17 ${hue})`)
  root.style.setProperty("--accent-foreground",    "oklch(1 0 0)")
  root.style.setProperty("--ring",                 `oklch(${Lring.toFixed(3)} 0.14 ${hue})`)
  root.style.setProperty("--sidebar-primary",      `oklch(${Lring.toFixed(3)} 0.15 ${hue})`)
  root.style.setProperty("--sidebar-ring",         `oklch(${Lring.toFixed(3)} 0.15 ${hue})`)
}

function resetPrimary() {
  const props = ["--primary", "--primary-foreground", "--accent", "--accent-foreground", "--ring", "--sidebar-primary", "--sidebar-ring"]
  props.forEach(p => document.documentElement.style.removeProperty(p))
}

// bgBrightness: 0–100 → lightness 0.12–0.99 (чёрный → белый)
function bgBrightnessToL(b: number) {
  return 0.12 + (b / 100) * 0.87
}

// bgSaturation: 0–100 → chroma multiplier 0–2 (50 = default = 1.0)
function bgSaturationToMult(s: number) {
  return s / 50
}

function applyBgHue(hue: number, brightness: number, saturation: number) {
  const L      = bgBrightnessToL(brightness)
  const m      = bgSaturationToMult(saturation)
  // Карточки заметно светлее фона (как белые окна на сером фоне в светлой теме)
  const Lcard  = Math.min(L + 0.08, 0.99)
  const Lmuted = Math.max(L - 0.03, 0.4)
  const Linput = Math.max(L - 0.05, 0.4)
  const root = document.documentElement
  root.style.setProperty("--background",     `oklch(${L.toFixed(3)} ${(0.025 * m).toFixed(4)} ${hue})`)
  root.style.setProperty("--secondary",      `oklch(${L.toFixed(3)} ${(0.025 * m).toFixed(4)} ${hue})`)
  root.style.setProperty("--muted",          `oklch(${Lmuted.toFixed(3)} ${(0.018 * m).toFixed(4)} ${hue})`)
  root.style.setProperty("--input",          `oklch(${Linput.toFixed(3)} ${(0.018 * m).toFixed(4)} ${hue})`)
  root.style.setProperty("--sidebar",        `oklch(${Math.min(L + 0.05, 0.99).toFixed(3)} ${(0.018 * m).toFixed(4)} ${hue})`)
  root.style.setProperty("--sidebar-accent", `oklch(${Lmuted.toFixed(3)} ${(0.018 * m).toFixed(4)} ${hue})`)
  root.style.setProperty("--card",           `oklch(${Lcard.toFixed(3)} ${(0.008 * m).toFixed(4)} ${hue})`)
  root.style.setProperty("--popover",        `oklch(${Lcard.toFixed(3)} ${(0.008 * m).toFixed(4)} ${hue})`)
}

function resetBgHue() {
  const props = ["--background", "--secondary", "--muted", "--input", "--sidebar", "--sidebar-accent", "--card", "--popover"]
  props.forEach(p => document.documentElement.style.removeProperty(p))
}

export function useCustomTheme() {
  const [hue,           setHueState]           = useState<number | null>(null)
  const [brightness,    setBrightnessState]    = useState<number | null>(null)
  const [bgHue,         setBgHueState]         = useState<number | null>(null)
  const [bgBrightness,  setBgBrightnessState]  = useState<number | null>(null)
  const [bgSaturation,  setBgSaturationState]  = useState<number | null>(null)

  useEffect(() => {
    if (typeof window === "undefined") return

    const storedHue           = localStorage.getItem(CUSTOM_HUE_KEY)
    const storedBrightness    = localStorage.getItem(CUSTOM_BRIGHTNESS_KEY)
    const storedBgHue         = localStorage.getItem(CUSTOM_BG_HUE_KEY)
    const storedBgBrightness  = localStorage.getItem(CUSTOM_BG_BRIGHTNESS_KEY)
    const storedBgSaturation  = localStorage.getItem(CUSTOM_BG_SATURATION_KEY)

    const h = storedHue        !== null ? Number(storedHue)        : DEFAULT_HUE
    const b = storedBrightness !== null ? Number(storedBrightness) : DEFAULT_BRIGHTNESS

    if (storedHue !== null || storedBrightness !== null) {
      if (storedHue !== null) setHueState(h)
      if (storedBrightness !== null) setBrightnessState(b)
      applyPrimary(h, b)
    }

    if (storedBgHue !== null || storedBgBrightness !== null || storedBgSaturation !== null) {
      const bg  = storedBgHue         !== null ? Number(storedBgHue)         : DEFAULT_BG_HUE
      const bgb = storedBgBrightness  !== null ? Number(storedBgBrightness)  : DEFAULT_BG_BRIGHTNESS
      const bgs = storedBgSaturation  !== null ? Number(storedBgSaturation)  : DEFAULT_BG_SATURATION
      if (storedBgHue        !== null) setBgHueState(bg)
      if (storedBgBrightness !== null) setBgBrightnessState(bgb)
      if (storedBgSaturation !== null) setBgSaturationState(bgs)
      applyBgHue(bg, bgb, bgs)
    }
  }, [])

  const setHue = useCallback((h: number | null) => {
    setHueState(h)
    const b = Number(localStorage.getItem(CUSTOM_BRIGHTNESS_KEY) ?? DEFAULT_BRIGHTNESS)
    if (h === null) {
      localStorage.removeItem(CUSTOM_HUE_KEY)
      const storedB = localStorage.getItem(CUSTOM_BRIGHTNESS_KEY)
      if (storedB === null) resetPrimary()
      else applyPrimary(DEFAULT_HUE, Number(storedB))
    } else {
      localStorage.setItem(CUSTOM_HUE_KEY, String(h))
      applyPrimary(h, b)
    }
  }, [])

  const setBrightness = useCallback((b: number | null) => {
    setBrightnessState(b)
    const h = Number(localStorage.getItem(CUSTOM_HUE_KEY) ?? DEFAULT_HUE)
    if (b === null) {
      localStorage.removeItem(CUSTOM_BRIGHTNESS_KEY)
      const storedH = localStorage.getItem(CUSTOM_HUE_KEY)
      if (storedH === null) resetPrimary()
      else applyPrimary(Number(storedH), DEFAULT_BRIGHTNESS)
    } else {
      localStorage.setItem(CUSTOM_BRIGHTNESS_KEY, String(b))
      applyPrimary(h, b)
    }
  }, [])

  const setBgHue = useCallback((h: number | null) => {
    setBgHueState(h)
    const bgb = Number(localStorage.getItem(CUSTOM_BG_BRIGHTNESS_KEY) ?? DEFAULT_BG_BRIGHTNESS)
    const bgs = Number(localStorage.getItem(CUSTOM_BG_SATURATION_KEY) ?? DEFAULT_BG_SATURATION)
    if (h === null) {
      localStorage.removeItem(CUSTOM_BG_HUE_KEY)
      const storedBgb = localStorage.getItem(CUSTOM_BG_BRIGHTNESS_KEY)
      const storedBgs = localStorage.getItem(CUSTOM_BG_SATURATION_KEY)
      if (storedBgb === null && storedBgs === null) resetBgHue()
      else applyBgHue(DEFAULT_BG_HUE, bgb, bgs)
    } else {
      localStorage.setItem(CUSTOM_BG_HUE_KEY, String(h))
      applyBgHue(h, bgb, bgs)
    }
  }, [])

  const setBgBrightness = useCallback((b: number | null) => {
    setBgBrightnessState(b)
    const h   = Number(localStorage.getItem(CUSTOM_BG_HUE_KEY)        ?? DEFAULT_BG_HUE)
    const bgs = Number(localStorage.getItem(CUSTOM_BG_SATURATION_KEY) ?? DEFAULT_BG_SATURATION)
    if (b === null) {
      localStorage.removeItem(CUSTOM_BG_BRIGHTNESS_KEY)
      const storedH   = localStorage.getItem(CUSTOM_BG_HUE_KEY)
      const storedBgs = localStorage.getItem(CUSTOM_BG_SATURATION_KEY)
      if (storedH === null && storedBgs === null) resetBgHue()
      else applyBgHue(h, DEFAULT_BG_BRIGHTNESS, bgs)
    } else {
      localStorage.setItem(CUSTOM_BG_BRIGHTNESS_KEY, String(b))
      applyBgHue(h, b, bgs)
    }
  }, [])

  const setBgSaturation = useCallback((s: number | null) => {
    setBgSaturationState(s)
    const h   = Number(localStorage.getItem(CUSTOM_BG_HUE_KEY)        ?? DEFAULT_BG_HUE)
    const bgb = Number(localStorage.getItem(CUSTOM_BG_BRIGHTNESS_KEY) ?? DEFAULT_BG_BRIGHTNESS)
    if (s === null) {
      localStorage.removeItem(CUSTOM_BG_SATURATION_KEY)
      const storedH   = localStorage.getItem(CUSTOM_BG_HUE_KEY)
      const storedBgb = localStorage.getItem(CUSTOM_BG_BRIGHTNESS_KEY)
      if (storedH === null && storedBgb === null) resetBgHue()
      else applyBgHue(h, bgb, DEFAULT_BG_SATURATION)
    } else {
      localStorage.setItem(CUSTOM_BG_SATURATION_KEY, String(s))
      applyBgHue(h, bgb, s)
    }
  }, [])

  return { hue, setHue, brightness, setBrightness, bgHue, setBgHue, bgBrightness, setBgBrightness, bgSaturation, setBgSaturation }
}

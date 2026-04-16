"use client"

import { useTheme } from "next-themes"
import { useEffect, useState } from "react"
import { FlowerEffect } from "./flower-effect"

export function FlowerWrapper() {
    const { theme } = useTheme()
    const [mounted, setMounted] = useState(false)

    useEffect(() => {
        setMounted(true)
    }, [])

    if (!mounted || theme !== "march") return null

    return <FlowerEffect />
}

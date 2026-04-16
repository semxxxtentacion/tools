"use client"

import { useEffect, useRef } from "react"

interface Petal {
    x: number
    y: number
    size: number
    speed: number
    opacity: number
    drift: number
    rotation: number
    rotationSpeed: number
    colorIndex: number
}

const PETAL_COLORS = [
    [255, 182, 193],  // light pink
    [255, 105, 135],  // deep pink
    [255, 148, 172],  // rose
    [220, 120, 160],  // mauve
    [255, 200, 210],  // pale pink
]

function drawFlower(
    ctx: CanvasRenderingContext2D,
    petal: Petal,
) {
    const [r, g, b] = PETAL_COLORS[petal.colorIndex]

    ctx.save()
    ctx.translate(petal.x, petal.y)
    ctx.rotate(petal.rotation)
    ctx.globalAlpha = petal.opacity

    // 5 petals
    for (let i = 0; i < 5; i++) {
        ctx.save()
        ctx.rotate((i * 2 * Math.PI) / 5)
        ctx.fillStyle = `rgb(${r}, ${g}, ${b})`
        ctx.beginPath()
        ctx.ellipse(0, -petal.size * 0.75, petal.size * 0.38, petal.size * 0.75, 0, 0, Math.PI * 2)
        ctx.fill()
        ctx.restore()
    }

    // Yellow center
    ctx.fillStyle = `rgba(255, 220, 80, ${petal.opacity})`
    ctx.beginPath()
    ctx.arc(0, 0, petal.size * 0.28, 0, Math.PI * 2)
    ctx.fill()

    ctx.restore()
}

export function FlowerEffect() {
    const canvasRef = useRef<HTMLCanvasElement>(null)
    const animFrameRef = useRef<number | undefined>(undefined)
    const petalsRef = useRef<Petal[]>([])

    useEffect(() => {
        const canvas = canvasRef.current
        if (!canvas) return

        const ctx = canvas.getContext("2d", { alpha: true })
        if (!ctx) return

        const resize = () => {
            const dpr = window.devicePixelRatio || 1
            const w = window.innerWidth
            const h = window.innerHeight
            canvas.width = w * dpr
            canvas.height = h * dpr
            canvas.style.width = `${w}px`
            canvas.style.height = `${h}px`
            ctx.scale(dpr, dpr)
        }

        resize()

        let resizeTimer: ReturnType<typeof setTimeout>
        const handleResize = () => {
            clearTimeout(resizeTimer)
            resizeTimer = setTimeout(resize, 100)
        }
        window.addEventListener("resize", handleResize, { passive: true })

        const createPetal = (): Petal => ({
            x: Math.random() * window.innerWidth,
            y: -20,
            size: Math.random() * 7 + 4,
            speed: Math.random() * 1.2 + 0.4,
            opacity: Math.random() * 0.5 + 0.35,
            drift: Math.random() * 0.6 - 0.3,
            rotation: Math.random() * Math.PI * 2,
            rotationSpeed: (Math.random() - 0.5) * 0.04,
            colorIndex: Math.floor(Math.random() * PETAL_COLORS.length),
        })

        const count = Math.min(
            Math.floor((window.innerWidth * window.innerHeight) / 18000),
            55
        )
        petalsRef.current = Array.from({ length: count }, createPetal).map((p, i) => ({
            ...p,
            y: Math.random() * window.innerHeight,
        }))

        const animate = () => {
            ctx.clearRect(0, 0, window.innerWidth, window.innerHeight)

            for (const p of petalsRef.current) {
                p.y += p.speed
                p.x += p.drift
                p.rotation += p.rotationSpeed

                if (p.y > window.innerHeight + 20) {
                    p.y = -20
                    p.x = Math.random() * window.innerWidth
                }
                if (p.x < -30) p.x = window.innerWidth + 30
                if (p.x > window.innerWidth + 30) p.x = -30

                drawFlower(ctx, p)
            }

            animFrameRef.current = requestAnimationFrame(animate)
        }

        animFrameRef.current = requestAnimationFrame(animate)

        const handleVisibility = () => {
            if (document.hidden) {
                if (animFrameRef.current) {
                    cancelAnimationFrame(animFrameRef.current)
                    animFrameRef.current = undefined
                }
            } else if (!animFrameRef.current) {
                animFrameRef.current = requestAnimationFrame(animate)
            }
        }
        document.addEventListener("visibilitychange", handleVisibility)

        return () => {
            window.removeEventListener("resize", handleResize)
            document.removeEventListener("visibilitychange", handleVisibility)
            if (animFrameRef.current) cancelAnimationFrame(animFrameRef.current)
            clearTimeout(resizeTimer)
        }
    }, [])

    return (
        <canvas
            ref={canvasRef}
            className="fixed inset-0 pointer-events-none z-[9999]"
            style={{
                willChange: "transform",
                transform: "translate3d(0, 0, 0)",
                backfaceVisibility: "hidden",
            }}
        />
    )
}

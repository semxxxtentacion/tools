"use client"

import React, { useRef } from "react"
import { useSwipeable } from "react-swipeable"
import { useRouter, usePathname } from "next/navigation"
import { motion, AnimatePresence } from "framer-motion"

// Массив путей
const TABS_ORDER = [
  '/qr-scanner/',
  '/schedule/',
  '/',
  '/performance/',
  '/reviews/',
  '/students/'
]

export function SwipeTransition({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  let pathname = usePathname()
  
  // Добавляем слэш в конец для корректного поиска в массиве
  if (pathname !== '/' && !pathname.endsWith('/')) {
    pathname = pathname + '/'
  }

  // --- ИСПРАВЛЕНИЕ 1: Умное направление (для футера и свайпов) ---
  const prevPathRef = useRef(pathname)
  const directionRef = useRef(1)

  // Если путь изменился (по клику или свайпу), вычисляем куда мы движемся
  if (pathname !== prevPathRef.current) {
    const oldIdx = TABS_ORDER.indexOf(prevPathRef.current)
    const newIdx = TABS_ORDER.indexOf(pathname)
    
    // Если обе вкладки есть в футере, вычисляем направление (вправо или влево)
    if (oldIdx !== -1 && newIdx !== -1) {
      directionRef.current = newIdx > oldIdx ? 1 : -1
    }
    prevPathRef.current = pathname
  }

  const direction = directionRef.current
  const currentIndex = TABS_ORDER.indexOf(pathname)

  // Обработчики свайпов (теперь они просто меняют URL, а направление посчитается само)
  const handlers = useSwipeable({
    onSwipedLeft: () => {
      if (currentIndex !== -1 && currentIndex < TABS_ORDER.length - 1) {
        router.push(TABS_ORDER[currentIndex + 1])
      }
    },
    onSwipedRight: () => {
      if (currentIndex > 0) {
        router.push(TABS_ORDER[currentIndex - 1])
      }
    },
    preventScrollOnSwipe: true,
    trackMouse: false, 
    delta: 40, 
  })

  // --- ИСПРАВЛЕНИЕ 2: Оптимизация для слабых телефонов ---
  const variants = {
    enter: (dir: number) => ({
      x: dir > 0 ? "100%" : "-100%",
      opacity: 0,
    }),
    center: {
      x: "0%",
      opacity: 1,
    },
    exit: (dir: number) => ({
      x: dir < 0 ? "100%" : "-100%",
      opacity: 0,
    }),
  }

  // Если мы не в футере, отдаем обычную страницу без анимации
  if (currentIndex === -1) {
    return <>{children}</>
  }

  return (
    <div {...handlers} className="w-full relative overflow-x-hidden touch-pan-y" style={{ minHeight: 'calc(100vh - 80px)' }}>
      {/* Убрали тяжелый popLayout */}
      <AnimatePresence initial={false} custom={direction}>
        <motion.div
          key={pathname}
          custom={direction}
          variants={variants}
          initial="enter"
          animate="center"
          exit="exit"
          // Жесткое позиционирование, чтобы старая страница не сдвигала новую
          className="absolute top-0 left-0 w-full h-full"
          // Форсируем GPU-ускорение браузера
          style={{ willChange: "transform, opacity" }}
          transition={{
            type: "tween", // Простая и легкая линейная анимация вместо "spring"
            ease: "easeInOut",
            duration: 0.25 // Идеальная скорость перелистывания (250мс)
          }}
        >
          {children}
        </motion.div>
      </AnimatePresence>
    </div>
  )
}

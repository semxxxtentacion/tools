"use client"

import type React from "react"

import { useAuth } from "@/hooks/use-auth"
import { useAdmin } from "@/hooks/use-admin"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { SignUpForm } from "@/components/auth/sign-up-form"
import { TelegramWidget, TelegramAuthError } from "@/components/auth/telegram-widget"
import { BottomNavigation } from "./layout/bottom-navigation"
import { MaintenancePage } from "./maintenance-page"

interface RouteGuardProps {
  children: React.ReactNode
}

export function RouteGuard({ children }: RouteGuardProps) {
  const { 
    user, 
    isLoading, 
    isAuthenticated, 
    isTelegramWebApp, 
    isTelegramWidget, 
    setWebAppUser,
    signUp,
  } = useAuth()
  const { isAdmin } = useAdmin()

  const handleTelegramAuth = async (telegramUser: any) => {
    try {
      await setWebAppUser(telegramUser)
    } catch (error) {
      console.error("Failed to set web app user:", error)
    }
  }

  if (isLoading) {
    return (
      <div className="min-h-screen bg-background relative z-10">
        <div className="flex items-center justify-center min-h-[calc(100vh-3.5rem)]">
          <LoadingSpinner />
        </div>
      </div>
    )
  }

  if (isAuthenticated) {
    // if (!isAdmin) {
    //   return <MaintenancePage />
    // }

    return <>
      <div className="pb-16 pb-safe relative z-10">
        {children}
      </div>

      <BottomNavigation/>
    </>
  }

  const devTestUserId = process.env.NEXT_PUBLIC_DEV_AUTH_USER_ID
  const devTestHash = process.env.NEXT_PUBLIC_DEV_AUTH_HASH
  const devTestLogin = process.env.NEXT_PUBLIC_DEV_AUTH_LOGIN
  const devTestPassword = process.env.NEXT_PUBLIC_DEV_AUTH_PASSWORD
  const devTestUser =
    devTestUserId && devTestHash
      ? { id: Number(devTestUserId), hash: devTestHash }
      : null
  const showDevAuth =
    (process.env.NODE_ENV === "development" ||
      ["true", "1"].includes(
        String(process.env.NEXT_PUBLIC_ENABLE_DEV_AUTH ?? "").toLowerCase()
      )) &&
    !!devTestUser &&
    !!devTestLogin &&
    !!devTestPassword

  // If user is in Telegram WebApp but not registered, show sign up form + тестовый вход в dev
  if (isTelegramWebApp) {
    return (
      <div className="min-h-screen bg-background relative z-10 flex flex-col items-center justify-center p-4">
        <SignUpForm />
        {showDevAuth && (
          <div className="mt-4 text-center">
            <button
              type="button"
              onClick={async () => {
                if (!devTestUser || !devTestLogin || !devTestPassword) return
                await setWebAppUser(devTestUser)
                try {
                  await signUp(devTestLogin, devTestPassword)
                } catch (e) {
                  console.error("Тестовый signUp failed:", e)
                }
              }}
              className="text-sm text-muted-foreground underline hover:text-foreground"
            >
              Тестовый вход (без Telegram)
            </button>
          </div>
        )}
      </div>
    )
  }

  // If user is in regular browser and not authenticated via Telegram Widget — только виджет
  if (!isTelegramWidget) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center p-4 relative z-10">
        <div className="w-full max-w-md">
          <TelegramWidget 
            onAuth={handleTelegramAuth} 
            botName={process.env.TELEGRAM_BOT_NAME || "your_bot_name"}
          />
        </div>
      </div>
    )
  }

  // If user is authenticated via Telegram Widget but not registered, show sign up form
  return (
    <div className="min-h-screen bg-background relative z-10">
      <SignUpForm />
    </div>
  )
}

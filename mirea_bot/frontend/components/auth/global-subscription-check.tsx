"use client"

import { useAuth } from "@/hooks/use-auth"
import { SubscriptionCheck } from "@/components/auth/subscription-check"
import type React from "react"

export function GlobalSubscriptionCheck({ children }: { children: React.ReactNode }) {
    const { user, isAuthenticated } = useAuth()

    if (isAuthenticated && user && user.is_subscribed === false) {
        return <SubscriptionCheck />
    }

    return <>{children}</>
}
"use client"

import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { AlertCircle, ExternalLink, RefreshCw, ShieldAlert } from "lucide-react"
import { apiClient } from "@/lib/api"
import { useAuth } from "@/hooks/use-auth"
import { getTelegramWebApp } from "@/lib/telegram"

export function SubscriptionCheck() {
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState("")
    const { refreshUser } = useAuth()
    const webApp = getTelegramWebApp()

    const handleCheck = async () => {
        setLoading(true)
        setError("")
        try {
            const res = await apiClient.checkSubscription()
            if (res.data.is_subscribed) {
                await refreshUser()
                webApp?.HapticFeedback?.notificationOccurred("success")
            } else {
                setError("Вы еще не подписались на канал. Проверьте подписку и попробуйте снова.")
                webApp?.HapticFeedback?.notificationOccurred("error")
            }
        } catch {
            setError("Ошибка при проверке. Убедитесь, что бот является администратором канала.")
            webApp?.HapticFeedback?.notificationOccurred("error")
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="flex items-center justify-center p-4 min-h-screen bg-background">
            <Card className="w-full max-w-md border-primary/20 shadow-lg shadow-primary/5 animate-fade-in">
                <CardHeader className="text-center pb-4">
                    <div className="mx-auto bg-primary/10 p-3 rounded-full mb-2 w-fit">
                        <ShieldAlert className="h-8 w-8 text-primary" />
                    </div>
                    <CardTitle className="text-xl font-bold">Доступ ограничен</CardTitle>
                    <CardDescription className="text-sm mt-2">
                        Для использования приложения необходимо быть подписчиком нашего канала
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4 pt-0">
                    {error && (
                        <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-lg flex items-start gap-2 animate-slide-up">
                            <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                            <span>{error}</span>
                        </div>
                    )}
                    <div className="space-y-3 pt-2">
                        <Button 
                            variant="outline" 
                            className="w-full border-primary/20 hover:bg-primary/5 text-primary"
                            onClick={() => window.open("https://t.me/mirea_tools", "_blank")}
                        >
                            <ExternalLink className="mr-2 h-4 w-4" />
                            Перейти в канал @mirea_tools
                        </Button>
                        <Button 
                            className="w-full font-medium" 
                            onClick={handleCheck}
                            disabled={loading}
                        >
                            {loading ? <RefreshCw className="mr-2 h-4 w-4 animate-spin" /> : "Я подписался"}
                        </Button>
                    </div>
                </CardContent>
            </Card>
        </div>
    )
}
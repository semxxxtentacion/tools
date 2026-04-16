"use client"

import type React from "react"

import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { useAuth } from "@/hooks/use-auth"
import { GraduationCap, Loader2 } from "lucide-react"

export function SignUpForm() {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState("")
  const [showOtpForm, setShowOtpForm] = useState(false)
  const [telegramHashForOtp, setTelegramHashForOtp] = useState("")
  const [otpCode, setOtpCode] = useState("")
  const [otpError, setOtpError] = useState("")
  const [otpLoading, setOtpLoading] = useState(false)
  const [otpTypeForOtp, setOtpTypeForOtp] = useState<string>("email")
  const { signUp, submitOtp, refreshUser, pendingOtpTelegramHash, pendingOtpType, clearPendingOtp } = useAuth()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError("")

    try {
      const result = await signUp(email, password)
      if (result?.otpRequired) {
        setTelegramHashForOtp(result.telegramHash)
        setOtpTypeForOtp(result.otpType ?? "email")
        setShowOtpForm(true)
      }
    } catch (error: any) {
      const errorMessage = error?.message || error?.toString() || ""
      if (errorMessage.includes("totp_secret_required") || errorMessage.includes("Требуется двухфакторная авторизация")) {
        setError("Авторизация по TOTP отключена. Пожалуйста, используйте подтверждение по почте или Max.")
      } else {
        setError("Ошибка регистрации. Проверьте данные и попробуйте снова.")
      }
    } finally {
      setIsLoading(false)
    }
  }

  const handleOtpSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const code = otpCode.replace(/\D/g, "").slice(0, 6)
    if (code.length !== 6) {
      setOtpError("Введите 6 цифр кода")
      return
    }
    setOtpLoading(true)
    setOtpError("")
    try {
      await submitOtp(otpHash, code)
    } catch (err: any) {
      const msg = err?.message || err?.toString() || ""
      if (msg.includes("Неверный код OTP") || msg.includes("otp_code_is_wrong")) {
        setOtpError("Код неверный. Проверьте и введите снова.")
      } else {
        setOtpError("Ошибка отправки кода. Попробуйте позже.")
      }
    } finally {
      setOtpLoading(false)
    }
  }

  const otpActive = showOtpForm || !!pendingOtpTelegramHash
  const otpHash = telegramHashForOtp || pendingOtpTelegramHash || ""
  const otpType = (otpTypeForOtp || pendingOtpType || "email").toLowerCase()
  const isMaxOtp = otpType === "max"
  console.log(otpType)
  const otpDescription = isMaxOtp
    ? "Введите 6-значный код авторизации отправленный вам в мессенджер Max"
    : "Введите 6-значный код авторизации отправленный вам на почту"

  if (otpActive && otpHash) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center p-4">
        <Card className="w-full max-w-md mx-auto">
          <CardHeader className="text-center">
            <CardTitle className="text-xl">Код подтверждения</CardTitle>
            <CardDescription>{otpDescription}</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleOtpSubmit} className="space-y-4">
              <div className="space-y-2">
                <Input
                  id="otp"
                  type="text"
                  inputMode="numeric"
                  maxLength={6}
                  placeholder="000000"
                  value={otpCode}
                  onChange={(e) => {
                    const v = e.target.value.replace(/\D/g, "").slice(0, 6)
                    setOtpCode(v)
                  }}
                  disabled={otpLoading}
                  className="text-center text-lg tracking-[0.5em] font-mono"
                  autoComplete="one-time-code"
                />
              </div>
              {otpError && (
                <Alert variant="destructive">
                  <AlertDescription>{otpError}</AlertDescription>
                </Alert>
              )}
              <Button type="submit" className="w-full" disabled={otpLoading || otpCode.length !== 6}>
                {otpLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Подтвердить
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="w-full"
                disabled={otpLoading}
                onClick={() => {
                  setShowOtpForm(false)
                  setTelegramHashForOtp("")
                  setOtpTypeForOtp("email")
                  setOtpCode("")
                  setOtpError("")
                  clearPendingOtp()
                }}
              >
                Назад
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4">
      <Card className="w-full max-w-md mx-auto">
        <CardHeader className="text-center">
          <div className="flex justify-center mb-4">
            <GraduationCap className="h-12 w-12 text-primary" />
          </div>
          <CardTitle className="text-2xl">Добро пожаловать</CardTitle>
          <CardDescription>Войдите в систему с помощью учетных данных МИРЭА</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">Email МИРЭА</Label>
              <Input
                id="email"
                type="email"
                placeholder="kupitman.i.n@edu.mirea.ru"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={isLoading}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Пароль</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                disabled={isLoading}
              />
            </div>
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Войти
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

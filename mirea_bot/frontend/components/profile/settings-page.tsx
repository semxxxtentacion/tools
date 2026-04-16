"use client"

import { useState, useEffect, useCallback } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Switch } from "@/components/ui/switch"
import { Progress } from "@/components/ui/progress"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import { apiClient, type Student, type CreateInviteResponse, type InviteInfo } from "@/lib/api"
import {
    Settings,
    UserPlus,
    ArrowLeft,
    CheckCircle,
    Mail,
    GraduationCap,
    Users,
    Moon,
    Sun,
    ExternalLink,
    Info,
    Lock,
    LogOut,
    AlertTriangle,
    Trash2,
    ChevronDown, Plus,
    Heart,
    Network,
    Shield,
    Loader2,
    Copy,
    Check,
    Flower2,
} from "lucide-react"
import { StudentInfoHeader } from "@/components/layout/student-info-header"
import { useTheme } from "next-themes"
import { ChangePasswordForm } from "./change-password-dialog"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { cn } from "@/lib/utils"
import { useAuth } from "@/hooks/use-auth"
import { useSnowSettings } from "@/hooks/use-snow-settings"
import { useCustomTheme, COLOR_PRESETS, BG_PRESETS, DEFAULT_HUE, DEFAULT_BG_HUE, DEFAULT_BRIGHTNESS, DEFAULT_BG_BRIGHTNESS, DEFAULT_BG_SATURATION } from "@/hooks/use-custom-theme"

function base32Decode(encoded: string): Uint8Array {
    const base32Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
    let bits = 0
    let value = 0
    let index = 0
    const output: number[] = []

    for (let i = 0; i < encoded.length; i++) {
        const char = encoded[i].toUpperCase()
        if (char === "=") break
        const charIndex = base32Chars.indexOf(char)
        if (charIndex === -1) continue

        value = (value << 5) | charIndex
        bits += 5

        if (bits >= 8) {
            output[index++] = (value >> (bits - 8)) & 0xff
            bits -= 8
        }
    }

    return new Uint8Array(output)
}

async function generateTotpCode(secret: string): Promise<string> {
    const time = Math.floor(Date.now() / 1000 / 30)

    const timeBuffer = new ArrayBuffer(8)
    const timeView = new DataView(timeBuffer)
    timeView.setBigUint64(0, BigInt(time), false)

    const keyBytes = base32Decode(secret)

    const key = await crypto.subtle.importKey(
        "raw",
        keyBytes,
        { name: "HMAC", hash: "SHA-1" },
        false,
        ["sign"]
    )

    const signature = await crypto.subtle.sign("HMAC", key, timeBuffer)
    const sigArray = new Uint8Array(signature)

    const offset = sigArray[sigArray.length - 1] & 0x0f
    const code = ((sigArray[offset] & 0x7f) << 24 |
        (sigArray[offset + 1] & 0xff) << 16 |
        (sigArray[offset + 2] & 0xff) << 8 |
        (sigArray[offset + 3] & 0xff)) % 1000000

    return code.toString().padStart(6, "0")
}

interface SettingsPageProps {
    onBack: () => void
}

export function SettingsPage({ onBack }: SettingsPageProps) {
    const [connectedStudents, setConnectedStudents] = useState<Student[]>([])
    const [connectedToUser, setConnectedToUser] = useState<Student[]>([])
    const [searchMode, setSearchMode] = useState<"email" | "tg">("email")
    const [searchEmail, setSearchEmail] = useState("")
    const [searchTg, setSearchTg] = useState("")
    const [invite, setInvite] = useState<CreateInviteResponse | null>(null)
    const [isCreatingInvite, setIsCreatingInvite] = useState(false)
    const [inviteCopied, setInviteCopied] = useState(false)
    const [foundStudent, setFoundStudent] = useState<Student | null>(null)
    const [isLoading, setIsLoading] = useState(false)
    const [isSearching, setIsSearching] = useState(false)
    const [error, setError] = useState("")
    const [success, setSuccess] = useState("")
    const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
    const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
    const [isDeleting, setIsDeleting] = useState(false)
    const [isConnectedStudentsOpen, setIsConnectedStudentsOpen] = useState(false)
    const [isConnectedToUserOpen, setIsConnectedToUserOpen] = useState(false)
    const [proxyValue, setProxyValue] = useState("")
    const [isSavingProxy, setIsSavingProxy] = useState(false)
    const [totpSecret, setTotpSecret] = useState<string | null>(null)
    const [totpCode, setTotpCode] = useState<string | null>(null)
    const [totpProgress, setTotpProgress] = useState(100)
    const [isTotpLoading, setIsTotpLoading] = useState(false)
    const [isCopied, setIsCopied] = useState(false)
    const { theme, setTheme } = useTheme()
    const { user, refreshUser } = useAuth()
    const { snowEnabled, setSnowEnabled } = useSnowSettings()
    const { hue, setHue, brightness, setBrightness, bgHue, setBgHue, bgBrightness, setBgBrightness, bgSaturation, setBgSaturation } = useCustomTheme()

    const loadTotpSecret = useCallback(async () => {
        if (!user?.has_totp_secret) return
        setIsTotpLoading(true)
        try {
            const response = await apiClient.getTotpSecret()
            setTotpSecret(response.data)
        } catch {
            // Секрет не настроен или ошибка
        } finally {
            setIsTotpLoading(false)
        }
    }, [user?.has_totp_secret])

    const copyTotpCode = async () => {
        if (!totpCode) return
        try {
            await navigator.clipboard.writeText(totpCode)
            setIsCopied(true)
            setTimeout(() => setIsCopied(false), 2000)
        } catch {
            // Clipboard API недоступен
        }
    }

    const loadConnectedStudents = async () => {
        setIsLoading(true)
        try {
            const response = await apiClient.listConnectedStudents()
            setConnectedStudents(response.data)
        } catch (error) {
            setError("Не удалось загрузить список студентов")
        } finally {
            setIsLoading(false)
        }
    }

    const loadConnectedToUser = async () => {
        setIsLoading(true)
        try {
            const response = await apiClient.listConnectedToUser()
            setConnectedToUser(response.data)
        } catch (error) {
            setError("Не удалось загрузить список пользователей")
        } finally {
            setIsLoading(false)
        }
    }

    const loadAll = async () => {
        await Promise.all([loadConnectedStudents(), loadConnectedToUser()])
    }

    useEffect(() => {
        if (searchMode !== "email" || !searchEmail.trim()) {
            setFoundStudent(null)
            return
        }

        const timeoutId = setTimeout(async () => {
            setIsSearching(true)
            setFoundStudent(null)
            setError("")

            try {
                const response = await apiClient.findStudent(searchEmail.trim())
                setFoundStudent(response.data)
            } catch (error) {
                setError("Студент не найден или произошла ошибка поиска")
            } finally {
                setIsSearching(false)
            }
        }, 500)

        return () => clearTimeout(timeoutId)
    }, [searchEmail, searchMode])

    useEffect(() => {
        if (searchMode !== "tg" || !searchTg.trim()) {
            setFoundStudent(null)
            return
        }

        const timeoutId = setTimeout(async () => {
            setIsSearching(true)
            setFoundStudent(null)
            setError("")

            try {
                const response = await apiClient.findStudentByTg(searchTg.trim())
                setFoundStudent(response.data)
            } catch (error) {
                setError("Студент не найден или произошла ошибка поиска")
            } finally {
                setIsSearching(false)
            }
        }, 500)

        return () => clearTimeout(timeoutId)
    }, [searchTg, searchMode])

    const connectStudent = async (email: string) => {
        try {
            await apiClient.connectStudent(email)
            setSuccess("Студент успешно добавлен")
            setIsAddDialogOpen(false)
            setSearchEmail("")
            setSearchTg("")
            setFoundStudent(null)
            await loadAll()
        } catch (error) {
            setError("Не удалось добавить студента. Возможно, он уже подключен.")
        }
    }

    const toggleStudent = async (email: string) => {
        try {
            await apiClient.toggleConnectedStudent(email)
            await loadConnectedStudents()
        } catch (error) {
            setError("Не удалось изменить статус студента")
        }
    }

    const disconnectStudent = async (email: string) => {
        try {
            await apiClient.disconnectStudent(email)
            setSuccess("Студент успешно отвязан")
            await loadAll()
        } catch (error) {
            setError("Не удалось отвязать студента")
        }
    }

    const disconnectFromUser = async (email: string) => {
        try {
            await apiClient.disconnectFromUser(email)
            setSuccess("Вы успешно отвязались от пользователя")
            await loadAll()
        } catch (error) {
            setError("Не удалось отвязаться от пользователя")
        }
    }

    const botName = process.env.TELEGRAM_BOT_NAME ?? "MireaQRBot"

    const handleCreateInvite = async () => {
        setIsCreatingInvite(true)
        try {
            const resp = await apiClient.createInvite()
            setInvite(resp.data)
        } catch {
            setError("Не удалось создать ссылку-приглашение")
        } finally {
            setIsCreatingInvite(false)
        }
    }

    const handleCopyInvite = async () => {
        if (!invite) return
        const link = `https://t.me/${botName}?startapp=${invite.token}`
        try {
            await navigator.clipboard.writeText(link)
            setInviteCopied(true)
            setTimeout(() => setInviteCopied(false), 2000)
        } catch {
            setError("Не удалось скопировать ссылку")
        }
    }

    const deleteUser = async () => {
        setIsDeleting(true)
        try {
            await apiClient.deleteUser()
            setSuccess("Аккаунт успешно удален")
            setIsDeleteDialogOpen(false)
            // Перенаправляем на страницу регистрации или закрываем приложение
            setTimeout(() => {
                window.location.href = "/"
            }, 2000)
        } catch (error) {
            setError("Не удалось удалить аккаунт")
        } finally {
            setIsDeleting(false)
        }
    }

    const saveProxy = async () => {
        setIsSavingProxy(true)
        setError("")
        try {
            await apiClient.updateProxy({ proxy: proxyValue.trim() })
            setSuccess("Прокси успешно обновлен")
            await refreshUser()
        } catch (error) {
            setError("Не удалось обновить прокси")
        } finally {
            setIsSavingProxy(false)
        }
    }

    const clearProxy = async () => {
        setIsSavingProxy(true)
        setError("")
        try {
            await apiClient.updateProxy({ proxy: "" })
            setProxyValue("")
            setSuccess("Прокси сброшен, будет использоваться стандартный")
            await refreshUser()
        } catch (error) {
            setError("Не удалось сбросить прокси")
        } finally {
            setIsSavingProxy(false)
        }
    }

    useEffect(() => {
        loadAll()
    }, [])

    useEffect(() => {
        loadTotpSecret()
    }, [loadTotpSecret])

    useEffect(() => {
        if (!totpSecret) {
            setTotpProgress(100)
            return
        }

        const updateCode = async () => {
            try {
                const code = await generateTotpCode(totpSecret)
                setTotpCode(code)
                setTotpProgress(100)
            } catch {
                console.error("Failed to generate TOTP code")
            }
        }

        updateCode()

        const progressInterval = setInterval(() => {
            const timeInPeriod = Math.floor(Date.now() / 1000) % 30
            const remaining = 30 - timeInPeriod
            setTotpProgress((remaining / 30) * 100)
        }, 1000)

        const codeInterval = setInterval(updateCode, 30000)

        return () => {
            clearInterval(progressInterval)
            clearInterval(codeInterval)
        }
    }, [totpSecret])

    useEffect(() => {
        if (user) {
            setProxyValue(user.custom_proxy || "")
        }
    }, [user])

    useEffect(() => {
        if (success) {
            const timer = setTimeout(() => setSuccess(""), 3000)
            return () => clearTimeout(timer)
        }
    }, [success])

    const getStudentWord = (count: number): string => {
        const lastDigit = count % 10
        const lastTwoDigits = count % 100

        // Исключения для 11-19
        if (lastTwoDigits >= 11 && lastTwoDigits <= 19) {
            return "студентов"
        }

        // Для чисел, оканчивающихся на 1 (кроме 11)
        if (lastDigit === 1) {
            return "студент"
        }

        // Для чисел, оканчивающихся на 2, 3, 4 (кроме 12, 13, 14)
        if (lastDigit >= 2 && lastDigit <= 4) {
            return "студента"
        }

        // Для остальных случаев (0, 5-9)
        return "студентов"
    }

    const renderStudentCard = (student: Student, showToggle: boolean = true, showBadge: boolean = true, onDelete?: (email: string) => void) => (
        <Card key={student.email} className="mb-2 p-0">
            <CardContent className="p-2">
                <div className="flex items-center justify-between gap-2">
                    <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                            <h3 className="font-semibold text-sm truncate">{student.fullname}</h3>
                        </div>
                        <div className="flex items-center gap-3 text-xs text-muted-foreground">
                            <div className="flex items-center gap-1">
                                <GraduationCap className="h-3 w-3" />
                                <span>{student.group}</span>
                            </div>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        {(student.enabled !== undefined && showBadge) && (
                            <Badge variant={student.enabled ? "default" : "secondary"} size="sm">
                                {student.enabled ? "Активен" : "Отключен"}
                            </Badge>
                        )}

                        {showToggle && student.enabled !== undefined && (
                            <Switch
                                checked={student.enabled}
                                onCheckedChange={() => toggleStudent(student.email)}
                                aria-label={`${student.enabled ? "Отключить" : "Включить"} ${student.fullname}`}
                            />
                        )}
                        
                        {onDelete && (
                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => onDelete(student.email)}
                                className="text-destructive hover:text-destructive h-8 w-8 p-0"
                            >
                                <Trash2 className="h-4 w-4" />
                            </Button>
                        )}
                    </div>
                </div>
            </CardContent>
        </Card>
    )

    const renderAddStudentDialog = () => (
        <Dialog open={isAddDialogOpen} onOpenChange={setIsAddDialogOpen}>
            <DialogTrigger asChild>
                <Button className="w-full">
                    <UserPlus className="mr-2 h-4 w-4" />
                    Добавить студента
                </Button>
            </DialogTrigger>
            <DialogContent className="max-w-md">
                <DialogHeader>
                    <DialogTitle>Добавить студента</DialogTitle>
                    <DialogDescription>Найдите студента и добавьте его в свой список</DialogDescription>
                </DialogHeader>
                <div className="space-y-4">
                    <div className="flex rounded-lg border overflow-hidden">
                        <button
                            type="button"
                            className={cn("flex-1 py-1.5 text-sm font-medium transition-colors", searchMode === "email" ? "bg-primary text-primary-foreground" : "hover:bg-muted")}
                            onClick={() => { setSearchMode("email"); setFoundStudent(null); setError("") }}
                        >
                            По email
                        </button>
                        <button
                            type="button"
                            className={cn("flex-1 py-1.5 text-sm font-medium transition-colors", searchMode === "tg" ? "bg-primary text-primary-foreground" : "hover:bg-muted")}
                            onClick={() => { setSearchMode("tg"); setFoundStudent(null); setError("") }}
                        >
                            По Telegram
                        </button>
                    </div>
                    <div className="space-y-2">
                        {searchMode === "email" ? (
                            <>
                                <Label htmlFor="search-email">Email студента</Label>
                                <Input
                                    id="search-email"
                                    type="email"
                                    placeholder="student@edu.mirea.ru"
                                    value={searchEmail}
                                    onChange={(e: { target: { value: string } }) => setSearchEmail(e.target.value)}
                                />
                            </>
                        ) : (
                            <>
                                <Label htmlFor="search-tg">Telegram username</Label>
                                <Input
                                    id="search-tg"
                                    type="text"
                                    placeholder="@username"
                                    value={searchTg}
                                    onChange={(e: { target: { value: string } }) => setSearchTg(e.target.value)}
                                />
                            </>
                        )}
                        {isSearching && (
                            <div className="flex items-center gap-2 text-sm text-muted-foreground">
                                <div className="animate-spin rounded-full h-3 w-3 border-b-2 border-current"></div>
                                <span>Поиск...</span>
                            </div>
                        )}
                    </div>

                    {foundStudent && (
                        <Card>
                            <CardContent className="p-4">
                                <div className="flex items-center justify-between">
                                    <div>
                                        <h4 className="font-semibold">{foundStudent.fullname}</h4>
                                        <p className="text-sm text-muted-foreground">{foundStudent.group}</p>
                                    </div>
                                    <Button onClick={() => connectStudent(foundStudent.email)} size="sm">
                                        <Plus />
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>
                    )}

                    {error && (
                        <Alert variant="destructive">
                            <AlertDescription>{error}</AlertDescription>
                        </Alert>
                    )}
                </div>
            </DialogContent>
        </Dialog>
    )

    const renderDeleteUserDialog = () => (
        <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
            <DialogContent className="max-w-md">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2 text-destructive">
                        <AlertTriangle className="h-5 w-5" />
                        Удаление аккаунта
                    </DialogTitle>
                    <DialogDescription>
                        Вы уверены, что хотите удалить свой аккаунт? Это действие нельзя отменить.
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-4">
                    <Alert variant="destructive">
                        <AlertTriangle className="h-4 w-4" />
                        <AlertDescription>
                            <strong>Внимание!</strong> При удалении аккаунта будут сброшены все ваши привязки к другим студентам. 
                            Другие студенты больше не смогут отмечать вашу посещаемость.
                        </AlertDescription>
                    </Alert>

                    <div className="flex gap-2 justify-end">
                        <Button
                            variant="outline"
                            onClick={() => setIsDeleteDialogOpen(false)}
                            disabled={isDeleting}
                        >
                            Отмена
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={deleteUser}
                            disabled={isDeleting}
                        >
                            {isDeleting ? (
                                <>
                                    <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2"></div>
                                    Удаление...
                                </>
                            ) : (
                                <>
                                    <LogOut className="mr-2 h-4 w-4" />
                                    Удалить аккаунт
                                </>
                            )}
                        </Button>
                    </div>
                </div>
            </DialogContent>
        </Dialog>
    )

    return (
        <div className="min-h-screen bg-background">
            <StudentInfoHeader />

            <main className="container px-4 py-6 space-y-6">
                {success && (
                    <Alert>
                        <CheckCircle className="h-4 w-4" />
                        <AlertDescription>{success}</AlertDescription>
                    </Alert>
                )}

                <Card>
                    <CardHeader>
                        <CardTitle className="text-lg flex items-center gap-2">
                            {theme === "dark" ? <Moon className="h-5 w-5" /> : theme === "march" ? <Flower2 className="h-5 w-5 text-pink-500" /> : <Sun className="h-5 w-5" />}
                            Внешний вид
                        </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3">
                        <Label className="text-sm font-medium">Тема оформления</Label>
                        <div className="grid grid-cols-3 gap-2">
                            <button
                                type="button"
                                onClick={() => setTheme("light")}
                                className={cn(
                                    "flex flex-col items-center gap-2 rounded-xl border-2 p-3 transition-colors",
                                    theme === "light"
                                        ? "border-primary bg-primary/10"
                                        : "border-border hover:border-muted-foreground/50"
                                )}
                            >
                                <Sun className="h-5 w-5" />
                                <span className="text-xs font-medium">Светлая</span>
                            </button>
                            <button
                                type="button"
                                onClick={() => setTheme("dark")}
                                className={cn(
                                    "flex flex-col items-center gap-2 rounded-xl border-2 p-3 transition-colors",
                                    theme === "dark"
                                        ? "border-primary bg-primary/10"
                                        : "border-border hover:border-muted-foreground/50"
                                )}
                            >
                                <Moon className="h-5 w-5" />
                                <span className="text-xs font-medium">Тёмная</span>
                            </button>
                            <button
                                type="button"
                                onClick={() => setTheme("march")}
                                className={cn(
                                    "flex flex-col items-center gap-2 rounded-xl border-2 p-3 transition-colors",
                                    theme === "march"
                                        ? "border-pink-400 bg-pink-50"
                                        : "border-border hover:border-pink-300"
                                )}
                            >
                                <Flower2 className={cn("h-5 w-5", theme === "march" ? "text-pink-500" : "text-muted-foreground")} />
                                <span className={cn("text-xs font-medium", theme === "march" ? "text-pink-600" : "")}>8 Марта</span>
                            </button>
                        </div>

                        <div className="pt-3 border-t space-y-4">
                            <div className="space-y-2">
                                <Label className="text-sm font-medium">Цвет акцента</Label>
                                <div className="flex flex-wrap gap-2">
                                    {COLOR_PRESETS.map((preset) => (
                                        <button
                                            key={preset.hue}
                                            type="button"
                                            title={preset.label}
                                            onClick={() => setHue(preset.hue)}
                                            className={cn(
                                                "w-8 h-8 rounded-full border-2 transition-transform hover:scale-110",
                                                (hue ?? DEFAULT_HUE) === preset.hue
                                                    ? "border-foreground scale-110"
                                                    : "border-transparent"
                                            )}
                                            style={{ backgroundColor: `oklch(0.6 0.22 ${preset.hue})` }}
                                        />
                                    ))}
                                    {hue !== null && (
                                        <button
                                            type="button"
                                            title="Сбросить"
                                            onClick={() => setHue(null)}
                                            className="w-8 h-8 rounded-full border-2 border-dashed border-muted-foreground/50 flex items-center justify-center text-muted-foreground hover:border-foreground transition-colors text-xs"
                                        >
                                            ✕
                                        </button>
                                    )}
                                </div>
                                <div className="space-y-1">
                                    <label htmlFor="hue-slider" className="sr-only">Оттенок акцентного цвета</label>
                                    <input
                                        id="hue-slider"
                                        type="range"
                                        min={0}
                                        max={360}
                                        value={hue ?? DEFAULT_HUE}
                                        onChange={(e) => setHue(Number(e.target.value))}
                                        className="w-full h-3 rounded-full cursor-pointer appearance-none"
                                        style={{ background: `linear-gradient(to right, oklch(0.6 0.22 0), oklch(0.6 0.22 60), oklch(0.6 0.22 120), oklch(0.6 0.22 180), oklch(0.6 0.22 240), oklch(0.6 0.22 300), oklch(0.6 0.22 360))` }}
                                    />
                                    <p className="text-xs text-muted-foreground text-right">Оттенок: {hue ?? DEFAULT_HUE}°</p>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <Label className="text-sm font-medium">Яркость акцента</Label>
                                <div className="space-y-1">
                                    <label htmlFor="brightness-slider" className="sr-only">Яркость акцентного цвета</label>
                                    <input
                                        id="brightness-slider"
                                        type="range"
                                        min={0}
                                        max={100}
                                        value={brightness ?? DEFAULT_BRIGHTNESS}
                                        onChange={(e) => setBrightness(Number(e.target.value))}
                                        className="w-full h-3 rounded-full cursor-pointer appearance-none"
                                        style={{ background: `linear-gradient(to right, oklch(0.12 0.22 ${hue ?? DEFAULT_HUE}), oklch(0.54 0.22 ${hue ?? DEFAULT_HUE}), oklch(0.96 0.22 ${hue ?? DEFAULT_HUE}))` }}
                                    />
                                    <div className="flex justify-between text-xs text-muted-foreground">
                                        <span>Темнее</span>
                                        {brightness !== null && (
                                            <button type="button" onClick={() => setBrightness(null)} className="underline">сбросить</button>
                                        )}
                                        <span>Светлее</span>
                                    </div>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <Label className="text-sm font-medium">Оттенок фона</Label>
                                <div className="flex flex-wrap gap-2">
                                    {BG_PRESETS.map((preset) => (
                                        <button
                                            key={preset.hue}
                                            type="button"
                                            title={preset.label}
                                            onClick={() => setBgHue(preset.hue)}
                                            className={cn(
                                                "w-8 h-8 rounded-full border-2 transition-transform hover:scale-110",
                                                (bgHue ?? DEFAULT_BG_HUE) === preset.hue
                                                    ? "border-foreground scale-110"
                                                    : "border-transparent"
                                            )}
                                            style={{ backgroundColor: `oklch(0.92 0.025 ${preset.hue})` }}
                                        />
                                    ))}
                                    {bgHue !== null && (
                                        <button
                                            type="button"
                                            title="Сбросить"
                                            onClick={() => setBgHue(null)}
                                            className="w-8 h-8 rounded-full border-2 border-dashed border-muted-foreground/50 flex items-center justify-center text-muted-foreground hover:border-foreground transition-colors text-xs"
                                        >
                                            ✕
                                        </button>
                                    )}
                                </div>
                                <div className="space-y-1">
                                    <label htmlFor="bg-hue-slider" className="sr-only">Оттенок фона</label>
                                    <input
                                        id="bg-hue-slider"
                                        type="range"
                                        min={0}
                                        max={360}
                                        value={bgHue ?? DEFAULT_BG_HUE}
                                        onChange={(e) => setBgHue(Number(e.target.value))}
                                        className="w-full h-3 rounded-full cursor-pointer appearance-none"
                                        style={{ background: `linear-gradient(to right, oklch(0.95 0.02 0), oklch(0.95 0.02 60), oklch(0.95 0.02 120), oklch(0.95 0.02 180), oklch(0.95 0.02 240), oklch(0.95 0.02 300), oklch(0.95 0.02 360))` }}
                                    />
                                    <p className="text-xs text-muted-foreground text-right">Оттенок: {bgHue ?? DEFAULT_BG_HUE}°</p>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <Label className="text-sm font-medium">Яркость фона</Label>
                                <div className="space-y-1">
                                    <label htmlFor="bg-brightness-slider" className="sr-only">Яркость фона</label>
                                    <input
                                        id="bg-brightness-slider"
                                        type="range"
                                        min={0}
                                        max={100}
                                        value={bgBrightness ?? DEFAULT_BG_BRIGHTNESS}
                                        onChange={(e) => setBgBrightness(Number(e.target.value))}
                                        className="w-full h-3 rounded-full cursor-pointer appearance-none"
                                        style={{ background: `linear-gradient(to right, oklch(0.12 0.025 ${bgHue ?? DEFAULT_BG_HUE}), oklch(0.55 0.025 ${bgHue ?? DEFAULT_BG_HUE}), oklch(0.99 0.025 ${bgHue ?? DEFAULT_BG_HUE}))` }}
                                    />
                                    <div className="flex justify-between text-xs text-muted-foreground">
                                        <span>Темнее</span>
                                        {bgBrightness !== null && (
                                            <button type="button" onClick={() => setBgBrightness(null)} className="underline">сбросить</button>
                                        )}
                                        <span>Светлее</span>
                                    </div>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <Label className="text-sm font-medium">Насыщенность фона</Label>
                                <div className="space-y-1">
                                    <label htmlFor="bg-saturation-slider" className="sr-only">Насыщенность фона</label>
                                    <input
                                        id="bg-saturation-slider"
                                        type="range"
                                        min={0}
                                        max={100}
                                        value={bgSaturation ?? DEFAULT_BG_SATURATION}
                                        onChange={(e) => setBgSaturation(Number(e.target.value))}
                                        className="w-full h-3 rounded-full cursor-pointer appearance-none"
                                        style={{ background: `linear-gradient(to right, oklch(0.85 0 ${bgHue ?? DEFAULT_BG_HUE}), oklch(0.85 0.05 ${bgHue ?? DEFAULT_BG_HUE}))` }}
                                    />
                                    <div className="flex justify-between text-xs text-muted-foreground">
                                        <span>Серый</span>
                                        {bgSaturation !== null && (
                                            <button type="button" onClick={() => setBgSaturation(null)} className="underline">сбросить</button>
                                        )}
                                        <span>Насыщеннее</span>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle className="text-lg flex items-center gap-2">
                            <Lock className="h-5 w-5" />
                            Смена пароля
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-4">
                            <div>
                                <p className="text-sm text-muted-foreground mb-4">
                                    Если вы поменяли пароль от ЛК, то для работы бота, вы должны изменить его здесь
                                </p>
                                <ChangePasswordForm />
                            </div>
                        </div>
                    </CardContent>
                </Card>

                {user?.has_totp_secret && (
                    <Card>
                        <CardHeader>
                            <CardTitle className="text-lg flex items-center gap-2">
                                <Shield className="h-5 w-5" />
                                Двухфакторная авторизация
                            </CardTitle>
                            <CardDescription>
                                Текущий код подтверждения для МИРЭА
                            </CardDescription>
                        </CardHeader>
                        <CardContent>
                            {isTotpLoading ? (
                                <div className="flex items-center justify-center py-4">
                                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                                </div>
                            ) : totpCode ? (
                                <div className="space-y-3">
                                    <div className="flex items-center justify-center gap-3">
                                        <Badge variant="outline" className="text-2xl font-mono px-4 py-2 tracking-widest">
                                            {totpCode}
                                        </Badge>
                                    </div>
                                    <div className="space-y-1">
                                        <Progress value={totpProgress} className="w-full h-1.5" />
                                        <p className="text-xs text-muted-foreground text-center">
                                            Обновление через {Math.ceil((totpProgress / 100) * 30)} сек
                                        </p>
                                    </div>
                                </div>
                            ) : (
                                <p className="text-sm text-muted-foreground text-center py-2">
                                    Не удалось загрузить код
                                </p>
                            )}
                        </CardContent>
                    </Card>
                )}

                <Card>
                    <CardHeader>
                        <CardTitle className="text-lg flex items-center gap-2">
                            <Network className="h-5 w-5" />
                            Настройка прокси
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-4">
                            <div>
                                <p className="text-sm text-muted-foreground mb-4">
                                    Укажите свой прокси-сервер для подключения к МИРЭА. Если не указан, будут использоваться прокси разработчика из общего списка.
                                </p>
                                <div className="space-y-2">
                                    <Label htmlFor="proxy-input">Адрес прокси</Label>
                                    <Input
                                        id="proxy-input"
                                        type="text"
                                        placeholder="http://username:password@proxy.example.com:8080"
                                        value={proxyValue}
                                        onChange={(e) => setProxyValue(e.target.value)}
                                        disabled={isSavingProxy}
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        Формат: http://username:password@host:port или socks5://username:password@host:port
                                    </p>
                                </div>
                            </div>
                            <div className="flex gap-2">
                                <Button
                                    onClick={saveProxy}
                                    disabled={isSavingProxy}
                                    className="flex-1"
                                >
                                    {isSavingProxy ? (
                                        <>
                                            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2"></div>
                                            Сохранение...
                                        </>
                                    ) : (
                                        "Сохранить"
                                    )}
                                </Button>
                                {user?.custom_proxy && (
                                    <Button
                                        variant="outline"
                                        onClick={clearProxy}
                                        disabled={isSavingProxy}
                                    >
                                        Сбросить
                                    </Button>
                                )}
                            </div>
                            {user?.custom_proxy && (
                                <Alert>
                                    <Info className="h-4 w-4" />
                                    <AlertDescription>
                                        Текущий прокси: <code className="text-xs bg-muted px-1 py-0.5 rounded">{user.custom_proxy}</code>
                                    </AlertDescription>
                                </Alert>
                            )}
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle className="text-lg flex items-center gap-2">
                            <Info className="h-5 w-5" />О приложении
                        </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div>
                            <h4 className="font-medium mb-2">MIREA QR Bot</h4>
                            <p className="text-sm text-muted-foreground mb-4">
                                Приложение для автоматической отметки посещаемости в МИРЭА через QR-коды. Позволяет отмечать не только
                                себя, но и других студентов из вашей группы.
                            </p>
                        </div>

                        <div className="pt-4 border-t space-y-3">
                            <Button
                                variant="outline"
                                className="w-full bg-transparent"
                                onClick={() => window.open("https://t.me/mirea_tools", "_blank")}
                            >
                                <ExternalLink className="mr-2 h-4 w-4" />
                                Telegram канал с обновлениями
                            </Button>
                            <p className="text-xs text-muted-foreground text-center">
                                Следите за новостями и обновлениями приложения
                            </p>
                            
                            <Button
                                variant="outline"
                                className="w-full bg-transparent border-pink-200 dark:border-pink-800 hover:bg-pink-50 dark:hover:bg-pink-950"
                                onClick={() => window.open("https://t.me/tribute/app?startapp=dGHp", "_blank")}
                            >
                                <Heart className="mr-2 h-4 w-4 text-pink-500" />
                                Поддержать проект
                            </Button>
                            <p className="text-xs text-muted-foreground text-center">
                                Пожертвование поможет поддерживать развитие приложения
                            </p>
                        </div>
                    </CardContent>
                </Card>

                <Card className="border-destructive">
                    <CardHeader>
                        <CardTitle className="text-lg flex items-center gap-2 text-destructive">
                            <LogOut className="h-5 w-5" />
                            Удаление аккаунта
                        </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div>
                            <p className="text-sm text-muted-foreground mb-4">
                                Удаление аккаунта приведет к полному удалению всех ваших данных и связей с другими студентами.
                            </p>
                            <Button
                                variant="destructive"
                                className="w-full"
                                onClick={() => setIsDeleteDialogOpen(true)}
                            >
                                <LogOut className="mr-2 h-4 w-4" />
                                Удалить аккаунт
                            </Button>
                        </div>
                    </CardContent>
                </Card>

                {renderDeleteUserDialog()}
            </main>
        </div>
    )
}

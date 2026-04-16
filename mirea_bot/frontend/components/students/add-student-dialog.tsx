"use client"

import type React from "react"
import { useState } from "react"
import { useStudentConnections } from "@/hooks/use-student-connections"
import { apiClient, type CreateInviteResponse } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { AtSign, Check, Copy, ExternalLink, Link, Loader2, Mail, Plus, Search } from "lucide-react"
import { GroupmatesCard } from "@/components/students/groupmates-card"

const BOT_NAME = process.env.NEXT_PUBLIC_BOT_NAME ?? "MireaQRBot"

type Tab = "search" | "invite"

export function AddStudentDialog({ children }: { children: React.ReactNode }) {
  const { connectStudent } = useStudentConnections()
  const [isOpen, setIsOpen] = useState(false)
  const [tab, setTab] = useState<Tab>("search")

  // Search tab
  const [inputValue, setInputValue] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState("")

  // Invite tab
  const [invite, setInvite] = useState<CreateInviteResponse | null>(null)
  const [isCreatingInvite, setIsCreatingInvite] = useState(false)
  const [inviteCopied, setInviteCopied] = useState(false)

  const handleConnect = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!inputValue.trim()) return
    setIsLoading(true)
    setError("")
    try {
      await connectStudent(inputValue.trim())
      setInputValue("")
      setIsOpen(false)
    } catch {
      setError("Пользователь не найден или уже подключен")
    } finally {
      setIsLoading(false)
    }
  }

  const handleCreateInvite = async () => {
    setIsCreatingInvite(true)
    try {
      const resp = await apiClient.createInvite()
      setInvite(resp.data)
    } finally {
      setIsCreatingInvite(false)
    }
  }

  const handleCopyInvite = async () => {
    if (!invite) return
    const link = `https://t.me/${BOT_NAME}?startapp=${invite.token}`
    try {
      await navigator.clipboard.writeText(link)
      setInviteCopied(true)
      setTimeout(() => setInviteCopied(false), 2000)
    } catch {}
  }

  const handleShareInvite = () => {
    if (!invite) return
    const link = `https://t.me/${BOT_NAME}?startapp=${invite.token}`
    if ((window as any).Telegram?.WebApp?.openTelegramLink) {
      (window as any).Telegram.WebApp.openTelegramLink(`https://t.me/share/url?url=${encodeURIComponent(link)}`)
    } else {
      window.open(`https://t.me/share/url?url=${encodeURIComponent(link)}`, "_blank")
    }
  }

  const handleOpenChange = (open: boolean) => {
    setIsOpen(open)
    if (!open) {
      setInputValue("")
      setError("")
      setInvite(null)
      setTab("search")
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="sm:max-w-[425px] max-h-[90vh] overflow-y-auto overflow-x-hidden [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1">
            <Plus className="h-5 w-5" /> Добавить в друзья
          </DialogTitle>
        </DialogHeader>

        {/* Tabs */}
        <div className="flex gap-1 bg-muted rounded-lg p-1">
          <button
            onClick={() => setTab("search")}
            className={`flex-1 flex items-center justify-center gap-1.5 text-sm py-1.5 rounded-md transition-colors ${
              tab === "search" ? "bg-background shadow-sm font-medium" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Search className="h-3.5 w-3.5" /> Поиск
          </button>
          <button
            onClick={() => setTab("invite")}
            className={`flex-1 flex items-center justify-center gap-1 text-sm py-1.5 rounded-md transition-colors ${
              tab === "invite" ? "bg-background shadow-sm font-medium" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Link className="h-3.5 w-3.5" /> Ссылка-приглашение
          </button>
        </div>

        {tab === "search" ? (
          <div className="space-y-4">
            <form onSubmit={handleConnect} className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Введите почту или username друга, чтобы найти и добавить его.
              </p>
              <div className="relative">
                <Input
                  value={inputValue}
                  onChange={(e) => setInputValue(e.target.value)}
                  placeholder="example@edu.mirea.ru или @username"
                />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button type="submit" className="w-full" disabled={isLoading || !inputValue.trim()}>
                {isLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Plus className="mr-2 h-4 w-4" />}
                Отправить запрос
              </Button>
            </form>
            <div className="mt-4">
              <GroupmatesCard />
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              Создайте ссылку и отправьте другу — он автоматически добавится к вам в друзья.
            </p>
            {invite ? (
              <div className="space-y-2">
                <div className="rounded-md bg-muted p-3 text-center">
                  <p className="text-sm font-medium">Ссылка создана ✓</p>
                  <p className="text-xs text-muted-foreground mt-0.5">Действует 48 часов</p>
                </div>
                <Button className="w-full" onClick={handleCopyInvite}>
                  {inviteCopied ? <><Check className="mr-2 h-4 w-4" />Скопировано!</> : <><Copy className="mr-2 h-4 w-4" />Скопировать ссылку</>}
                </Button>
                <Button variant="outline" className="w-full" onClick={handleShareInvite}>
                  <ExternalLink className="mr-2 h-4 w-4" /> Поделиться в Telegram
                </Button>
                <Button variant="ghost" className="w-full text-muted-foreground" onClick={() => setInvite(null)}>
                  Создать новую ссылку
                </Button>
              </div>
            ) : (
              <Button className="w-full" onClick={handleCreateInvite} disabled={isCreatingInvite}>
                {isCreatingInvite
                  ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Создание...</>
                  : <><Link className="mr-2 h-4 w-4" />Создать ссылку-приглашение</>}
              </Button>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

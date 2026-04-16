"use client"

import { useEffect, useState } from "react"
import { useAuth } from "@/hooks/use-auth"
import { Plus, Map } from "lucide-react"
import { Button } from "@/components/ui/button"
import { AddStudentDialog } from "@/components/students/add-student-dialog"
import { ProfileDialog } from "@/components/profile/profile-dialog"
import { apiClient } from "@/lib/api"
import { BotMap } from "@/components/map/BotMap"

export function StudentInfoHeader() {
  const { user } = useAuth()
  const [hasPending, setHasPending] = useState(false)
  const [isMapOpen, setIsMapOpen] = useState(false)

  useEffect(() => {
    if (user) {
      apiClient.getPendingConnections()
        .then(res => {
          if (res.data && res.data.length > 0) {
            setHasPending(true)
          }
        })
        .catch(() => {})
    }
  }, [user])

  if (!user) return null

  return (
    <>
      <div className="bg-card p-3 border-b border-border/40 sticky top-0 z-50 shadow-sm">
        <div className="flex items-center justify-between gap-3 max-w-7xl mx-auto w-full">
          
          {/* ЛЕВАЯ ЧАСТЬ: Кнопка "Карта" с обновленными стилями */}
          <div className="flex items-center">
            <Button 
              variant="outline" 
              size="sm" 
              className="flex items-center gap-2 rounded-full px-4 h-9 font-medium border-2 border-primary bg-background text-foreground hover:bg-accent transition-all active:scale-95"
              onClick={() => setIsMapOpen(true)}
            >
              <Map className="h-4 w-4 text-primary" />
              <span>Карта</span>
            </Button>
          </div>

          {/* ПРАВАЯ ЧАСТЬ: Добавление друга и Профиль */}
          <div className="flex items-center gap-2 shrink-0">
            <AddStudentDialog>
              <Button size="sm" variant="default">
                <Plus className="h-4 w-4 sm:mr-2" />
                <span className="hidden sm:inline">Add Friend</span>
              </Button>
            </AddStudentDialog>
            <div className="relative inline-flex">
              <ProfileDialog />
              {hasPending && (
                <span className="absolute -top-1 -right-1 flex h-3 w-3">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-destructive opacity-75" />
                  <span className="relative inline-flex rounded-full h-3 w-3 bg-destructive border-2 border-background" />
                </span>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* МОДАЛЬНОЕ ОКНО КАРТЫ */}
      {isMapOpen && (
        <BotMap onClose={() => setIsMapOpen(false)} />
      )}
    </>
  )
}

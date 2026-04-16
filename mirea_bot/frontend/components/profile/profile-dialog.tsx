"use client"

import type React from "react"
import { useState, useEffect, useMemo } from "react"
import { useAuth } from "@/hooks/use-auth"
import { useStudentConnections } from "@/hooks/use-student-connections"
import { Button } from "@/components/ui/button"
import { LogOut, Trash2, User, GraduationCap, Check, X, Users } from "lucide-react"
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Switch } from "@/components/ui/switch"
import { apiClient, type Student } from "@/lib/api"

export function ProfileDialog() {
  const { user, signOut } = useAuth()
  const {
    connectedStudents,
    connectedToUser,
    pendingRequests,
    acceptRequest,
    declineRequest,
    disconnectStudent,
    disconnectFromUser,
    toggleStudent,
    loadAll,
  } = useStudentConnections()
  const [isOpen, setIsOpen] = useState(false)
  const [acceptingId, setAcceptingId] = useState<string | null>(null)
  const [decliningId, setDecliningId] = useState<string | null>(null)
  const [removingEmail, setRemovingEmail] = useState<string | null>(null)
  const [togglingEmail, setTogglingEmail] = useState<string | null>(null)

  useEffect(() => {
    if (isOpen) loadAll()
  }, [isOpen])

  const friends = useMemo<Student[]>(() => {
    const seen = new Set<string>()
    const result: Student[] = []
    for (const s of [...connectedStudents, ...connectedToUser]) {
      const key = s.email || s.id
      if (!seen.has(key)) {
        seen.add(key)
        result.push(s)
      }
    }
    return result
  }, [connectedStudents, connectedToUser])

  const handleLogout = () => {
    if (confirm("Выйти из аккаунта?")) signOut()
  }

  const handleDelete = async () => {
    if (confirm("Полностью удалить аккаунт? Это действие необратимо.")) {
      await apiClient.deleteUser()
      signOut()
    }
  }

  const handleAccept = async (id: string) => {
    setAcceptingId(id)
    try { await acceptRequest(id) } finally { setAcceptingId(null) }
  }

  const handleDecline = async (id: string) => {
    setDecliningId(id)
    try { await declineRequest(id) } finally { setDecliningId(null) }
  }

  const handleRemoveFriend = async (friend: Student) => {
    setRemovingEmail(friend.email)
    try {
      await Promise.allSettled([
        disconnectStudent(friend.email),
        disconnectFromUser(friend.email),
      ])
    } finally {
      setRemovingEmail(null)
    }
  }

  const handleToggle = async (friend: Student) => {
    setTogglingEmail(friend.email)
    try {
      await toggleStudent(friend.email)
    } finally {
      setTogglingEmail(null)
    }
  }

  if (!user) return null

  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger asChild>
        {/* Кнопка "Профиль" теперь полностью идентична кнопке "Карта" */}
        <Button 
          variant="outline" 
          size="sm" 
          className="flex items-center gap-2 rounded-full px-4 h-9 font-medium border-2 border-primary bg-background text-foreground hover:bg-accent transition-all active:scale-95"
        >
          <User className="h-4 w-4 text-primary" />
          <span>Профиль</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[425px] max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Профиль</DialogTitle>
        </DialogHeader>
        <div className="space-y-5 py-2">
          <div className="space-y-1 bg-secondary/30 p-4 rounded-lg">
            <h3 className="font-bold text-lg">{user.fullname}</h3>
            <p className="text-sm text-muted-foreground flex items-center gap-1">
              <GraduationCap className="h-4 w-4" /> {user.group}
            </p>
          </div>

          <Accordion type="multiple" defaultValue={["item-pending", "item-friends"]} className="w-full">
            {pendingRequests && pendingRequests.length > 0 && (
              <AccordionItem value="item-pending" className="border-primary/20">
                <AccordionTrigger className="text-primary font-medium">
                  Запросы в друзья ({pendingRequests.length})
                </AccordionTrigger>
                <AccordionContent>
                  <div className="space-y-3 pt-2">
                    {pendingRequests.map((req) => (
                      <div key={req.id} className="p-3 bg-secondary/20 rounded-md flex flex-col gap-2">
                        <div>
                          <span className="font-medium text-sm">{req.fullname || req.email}</span>
                          <p className="text-xs text-muted-foreground">{req.group}</p>
                        </div>
                        <div className="flex gap-2">
                          <Button size="sm" onClick={() => handleAccept(req.id)} disabled={acceptingId === req.id}
                            className="flex-1 bg-green-600 hover:bg-green-700 text-white">
                            <Check className="mr-1 h-4 w-4" />
                            {acceptingId === req.id ? "..." : "Принять"}
                          </Button>
                          <Button size="sm" variant="destructive" onClick={() => handleDecline(req.id)} disabled={decliningId === req.id}
                            className="flex-1">
                            <X className="mr-1 h-4 w-4" />
                            {decliningId === req.id ? "..." : "Отказать"}
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                </AccordionContent>
              </AccordionItem>
            )}

            <AccordionItem value="item-friends">
              <AccordionTrigger className="text-sm font-medium">
                <span className="flex items-center gap-2">
                  <Users className="h-4 w-4" /> Друзья ({friends.length})
                </span>
              </AccordionTrigger>
              <AccordionContent>
                {friends.length === 0 ? (
                  <p className="text-sm text-muted-foreground py-3 text-center">Нет друзей.</p>
                ) : (
                  <div className="space-y-2 pt-1">
                    {friends.map((friend) => (
                      <div key={friend.email} className="flex items-center justify-between p-2 bg-background/50 rounded-md gap-2">
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium truncate">{friend.fullname || friend.email}</p>
                          <p className="text-xs text-muted-foreground truncate">{friend.group}</p>
                        </div>
                        {friend.enabled !== undefined && (
                          <Switch
                            checked={friend.enabled}
                            disabled={togglingEmail === friend.email}
                            onCheckedChange={() => handleToggle(friend)}
                          />
                        )}
                        <Button
                          size="sm"
                          variant="ghost"
                          className="shrink-0 text-muted-foreground hover:text-destructive"
                          disabled={removingEmail === friend.email}
                          onClick={() => handleRemoveFriend(friend)}
                        >
                          <X className="h-4 w-4" />
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
              </AccordionContent>
            </AccordionItem>
          </Accordion>

          <div className="flex flex-col gap-3 pt-2 border-t">
            <Button variant="outline" onClick={handleLogout} className="w-full justify-start text-foreground">
              <LogOut className="mr-2 h-4 w-4" /> Выйти
            </Button>
            <Button variant="destructive" onClick={handleDelete} className="w-full justify-start">
              <Trash2 className="mr-2 h-4 w-4" /> Удалить аккаунт
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}

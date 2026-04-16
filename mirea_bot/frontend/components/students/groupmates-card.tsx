"use client"

import { useEffect, useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Users, Plus, Check, Loader2, ChevronDown } from "lucide-react"
import { apiClient, type Student } from "@/lib/api"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { cn } from "@/lib/utils"

export function GroupmatesCard() {
  const [groupmates, setGroupmates] = useState<Student[]>([])
  const [loading, setLoading] = useState(true)
  const [addedEmails, setAddedEmails] = useState<Record<string, boolean>>({})
  const [isOpen, setIsOpen] = useState(false)

  useEffect(() => {
    setLoading(true)
    apiClient.getGroupmates()
      .then(res => {
        setGroupmates(res.data || [])
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [])

  const handleAdd = async (email: string) => {
    try {
      await apiClient.connectStudent(email)
      setAddedEmails(prev => ({ ...prev, [email]: true }))
    } catch (e) {
      console.error("Ошибка при добавлении", e)
    }
  }

  if (loading) {
    return (
      <Card className="border-border">
        <CardHeader className="pb-4 flex flex-row items-center justify-between space-y-0">
          <div className="space-y-1">
            <CardTitle className="flex items-center gap-2 text-lg text-muted-foreground">
              <Users className="h-5 w-5" />
              Ваши одногруппники
            </CardTitle>
            <CardDescription>
              Ищем одногруппников...
            </CardDescription>
          </div>
          <Loader2 className="h-5 w-5 text-muted-foreground animate-spin" />
        </CardHeader>
      </Card>
    )
  }

  return (
    <Card className="animate-fade-in">
      <Collapsible
        open={isOpen}
        onOpenChange={setIsOpen}
      >
          <CollapsibleTrigger asChild>
            <div className="cursor-pointer select-none">
              <CardHeader className="py-3 flex flex-row items-center justify-between space-y-0">
                <div className="space-y-1">
                  <CardTitle className="flex items-center gap-2 text-lg">
                    <Users className="h-5 w-5" />
                    Ваши одногруппники
                  </CardTitle>
                  <CardDescription>
                    Добавьте своих одногруппников в друзья
                  </CardDescription>
                </div>
                <ChevronDown 
                  className={cn(
                    "h-5 w-5 text-muted-foreground transition-transform duration-200", 
                    isOpen && "rotate-180"
                  )} 
                />
              </CardHeader>
            </div>
          </CollapsibleTrigger>
        <CollapsibleContent className="data-[state=closed]:animate-collapsible-up data-[state=open]:animate-collapsible-down overflow-hidden">
          <CardContent className="pt-1 pb-2 border-t border-border/40">
            <div className="flex flex-col">
              {groupmates.length > 0 ? groupmates.map((mate, index) => (
                <div 
                  key={mate.email} 
                  className={cn(
                    "flex items-center justify-between py-1.5 pr-2",
                    index !== groupmates.length - 1 && "border-b border-border/40",
                  )}
                >
                  <div className="flex flex-col overflow-hidden mr-2">
                    <span className="font-medium text-sm truncate">{mate.fullname}</span>
                    <span className="text-xs text-muted-foreground truncate">{mate.email}</span>
                  </div>
                  
                  <Button 
                    size="icon" 
                    variant={addedEmails[mate.email] ? "secondary" : "default"}
                    onClick={() => handleAdd(mate.email)}
                    disabled={addedEmails[mate.email]}
                    className={cn(
                      "h-8 w-8 shrink-0 rounded-full transition-all",
                      addedEmails[mate.email] && "bg-green-500/10 text-green-600 hover:bg-green-500/20 hover:text-green-700 opacity-100 disabled:opacity-100"
                    )}
                  >
                    {addedEmails[mate.email] ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
                  </Button>
                </div>
              )) : (
                <p className="text-sm text-muted-foreground text-center py-4">
                  Все одногруппники уже добавлены или в вашей группе пока нет других пользователей бота.
                </p>
              )}
            </div>
          </CardContent>
        </CollapsibleContent>
      </Collapsible>
    </Card>
  )
}
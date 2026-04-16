"use client"

import { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { apiClient, type Note, type Lesson } from "@/lib/api"
import { format } from "date-fns"
import { ru } from "date-fns/locale"
import { Trash2 } from "lucide-react"

interface NoteModalProps {
  lesson: Lesson | null
  isOpen: boolean
  existingNote: Note | null
  onClose: () => void
  onSaved: (lessonUuid: string, note: Note | null) => void
}

export function NoteModal({ lesson, isOpen, existingNote, onClose, onSaved }: NoteModalProps) {
  const [noteText, setNoteText] = useState("")
  const [reminderEnabled, setReminderEnabled] = useState(false)
  const [reminderDate, setReminderDate] = useState("")
  const [reminderTime, setReminderTime] = useState("")
  const [isSaving, setIsSaving] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [error, setError] = useState("")

  // При открытии модалки заполняем поля из existingNote
  useEffect(() => {
    if (isOpen) {
      if (existingNote) {
        setNoteText(existingNote.note_text)
        setReminderEnabled(existingNote.reminder_enabled)
        if (existingNote.reminder_at > 0) {
          const d = new Date(existingNote.reminder_at * 1000)
          setReminderDate(format(d, "yyyy-MM-dd"))
          setReminderTime(format(d, "HH:mm"))
        } else {
          // По умолчанию — время начала пары
          if (lesson) {
            const d = new Date(lesson.start * 1000)
            setReminderDate(format(d, "yyyy-MM-dd"))
            setReminderTime(format(d, "HH:mm"))
          }
        }
      } else {
        setNoteText("")
        setReminderEnabled(false)
        // По умолчанию предлагаем время начала пары
        if (lesson) {
          const d = new Date(lesson.start * 1000)
          setReminderDate(format(d, "yyyy-MM-dd"))
          setReminderTime(format(d, "HH:mm"))
        }
      }
      setError("")
    }
  }, [isOpen, existingNote, lesson])

  const getReminderTimestamp = (): number => {
    if (!reminderEnabled || !reminderDate || !reminderTime) return 0
    const dt = new Date(`${reminderDate}T${reminderTime}:00`)
    if (isNaN(dt.getTime())) return 0
    return Math.floor(dt.getTime() / 1000)
  }

  const handleSave = async () => {
    if (!lesson) return
    if (!noteText.trim()) {
      setError("Введите текст заметки")
      return
    }

    const reminderAt = getReminderTimestamp()
    if (reminderEnabled && reminderAt === 0) {
      setError("Укажите корректную дату и время напоминания")
      return
    }
    if (reminderEnabled && reminderAt <= Math.floor(Date.now() / 1000)) {
      setError("Время напоминания должно быть в будущем")
      return
    }

    setIsSaving(true)
    setError("")
    try {
      const resp = await apiClient.upsertNote(lesson.uuid, noteText.trim(), reminderEnabled, reminderAt)
      onSaved(lesson.uuid, resp.data)
      onClose()
    } catch (e: any) {
      setError(e.message || "Не удалось сохранить заметку")
    } finally {
      setIsSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!lesson) return
    setIsDeleting(true)
    setError("")
    try {
      await apiClient.deleteNote(lesson.uuid)
      onSaved(lesson.uuid, null)
      onClose()
    } catch (e: any) {
      setError(e.message || "Не удалось удалить заметку")
    } finally {
      setIsDeleting(false)
    }
  }

  if (!lesson) return null

  const lessonTime = format(new Date(lesson.start * 1000), "HH:mm", { locale: ru })

  return (
    <Dialog open={isOpen} onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent className="max-w-sm mx-auto">
        <DialogHeader>
          <DialogTitle className="text-base leading-tight pr-6">
            {lesson.title}
            <span className="block text-sm font-normal text-muted-foreground mt-0.5">
              {lessonTime} · {lesson.auditorium}
            </span>
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="note-text">Заметка</Label>
            <Textarea
              id="note-text"
              placeholder="Что нужно не забыть..."
              value={noteText}
              onChange={(e) => setNoteText(e.target.value)}
              rows={4}
              maxLength={1000}
              className="resize-none"
            />
            <p className="text-xs text-muted-foreground text-right">{noteText.length}/1000</p>
          </div>

          <div className="flex items-center justify-between">
            <Label htmlFor="reminder-switch" className="cursor-pointer">
              Напоминание в Telegram
            </Label>
            <Switch
              id="reminder-switch"
              checked={reminderEnabled}
              onCheckedChange={setReminderEnabled}
            />
          </div>

          {reminderEnabled && (
            <div className="space-y-2">
              <Label>Дата и время напоминания</Label>
              <div className="flex gap-2">
                <input
                  type="date"
                  value={reminderDate}
                  onChange={(e) => setReminderDate(e.target.value)}
                  className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                />
                <input
                  type="time"
                  value={reminderTime}
                  onChange={(e) => setReminderTime(e.target.value)}
                  className="w-28 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                />
              </div>
            </div>
          )}

          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}
        </div>

        <DialogFooter className="flex-row gap-2 sm:justify-between">
          {existingNote && (
            <Button
              variant="ghost"
              size="icon"
              onClick={handleDelete}
              disabled={isDeleting || isSaving}
              className="text-destructive hover:text-destructive shrink-0"
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          )}
          <div className="flex gap-2 ml-auto">
            <Button variant="outline" onClick={onClose} disabled={isSaving || isDeleting}>
              Отмена
            </Button>
            <Button onClick={handleSave} disabled={isSaving || isDeleting}>
              {isSaving ? "Сохранение..." : "Сохранить"}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

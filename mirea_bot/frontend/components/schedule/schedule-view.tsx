"use client"

import { useState, useEffect } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { apiClient, type Lesson, type Note } from "@/lib/api"
import { Calendar, Clock, MapPin, User, Users, ChevronLeft, ChevronRight, Plus, StickyNote } from "lucide-react"
import { format, addDays, subDays, startOfWeek, endOfWeek, eachDayOfInterval, isSameDay } from "date-fns"
import { ru } from "date-fns/locale"
import { StudentInfoHeader } from "@/components/layout/student-info-header"
import { AttendanceModal } from "@/components/schedule/attendance-modal"
import { NoteModal } from "@/components/schedule/note-modal"

interface ScheduleViewProps {
  onBack: () => void
}

const getLessonTypeColor = (type: string) => {
  switch (type) {
    case "ЛК": return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
    case "ПР": return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
    case "ЛБ": return "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200"
    default: return "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200"
  }
}

const getLessonTypeText = (type: string) => {
  switch (type) {
    case "ЛК": return "Лекция"
    case "ПР": return "Практика"
    case "ЛБ": return "Лабораторная"
    default: return "Занятие"
  }
}

const getAttendedTypeColor = (attended: number) => {
  switch (attended) {
    case 3: return "default"
    case 2: return "secondary" 
    default: return "destructive"
  }
}

const getAttendedTypeText = (attended: number) => {
  switch (attended) {
    case 3: return "Отмечен"
    case 2: return "Уважительная"
    default: return "Пропуск"
  }
}

const formatTime = (timestamp: number) => format(new Date(timestamp * 1000), "HH:mm")
const formatDate = (date: Date) => format(date, "d MMMM", { locale: ru })
const formatWeekday = (date: Date) => format(date, "EEEE", { locale: ru })

export function ScheduleView({ onBack }: ScheduleViewProps) {
  const [selectedDate, setSelectedDate] = useState(new Date())
  const [lessons, setLessons] = useState<Lesson[]>([])
  const [weekLessons, setWeekLessons] = useState<Record<string, Lesson[]>>({})
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState("")
  const [activeTab, setActiveTab] = useState("day")
  const [selectedLesson, setSelectedLesson] = useState<Lesson | null>(null)
  const [isAttendanceModalOpen, setIsAttendanceModalOpen] = useState(false)
  
  const [notes, setNotes] = useState<Record<string, Note>>({})
  const [noteLesson, setNoteLesson] = useState<Lesson | null>(null)
  const [isNoteModalOpen, setIsNoteModalOpen] = useState(false)
  const [parentMode, setParentMode] = useState(false)
  const [secretClicks, setSecretClicks] = useState(0)

  useEffect(() => {
    if (typeof window !== 'undefined') {
      const savedMode = localStorage.getItem('mirea_parent_mode')
      if (savedMode === 'true') setParentMode(true)
    }
  }, [])

  const handleSecretTap = () => {
    setSecretClicks((prev) => {
      const next = prev + 1
      if (next >= 5) {
        setParentMode((currentMode) => {
          const newMode = !currentMode
          if (typeof window !== 'undefined') localStorage.setItem('mirea_parent_mode', String(newMode))
          return newMode
        })
        return 0 
      }
      return next
    })
  }

  const handleLessonClick = (lesson: Lesson) => {
    setSelectedLesson(lesson)
    setIsAttendanceModalOpen(true)
  }

  const handleNoteClick = (e: React.MouseEvent, lesson: Lesson) => {
    e.stopPropagation()
    setNoteLesson(lesson)
    setIsNoteModalOpen(true)
  }

  const handleNoteSaved = (lessonUuid: string, note: Note | null) => {
    setNotes((prev) => {
      const next = { ...prev }
      if (note) next[lessonUuid] = note
      else delete next[lessonUuid]
      return next
    })
  }

  const loadLessons = async (date: Date) => {
    setIsLoading(true)
    setError("")
    try {
      const response = await apiClient.getLessons(date.getFullYear(), date.getMonth() + 1, date.getDate())
      setLessons(response.data)
      loadNotesForLessons(response.data)
    } catch (error) {
      setError("Не удалось загрузить расписание")
    } finally {
      setIsLoading(false)
    }
  }

  const loadWeekLessons = async (date: Date) => {
    setIsLoading(true)
    setError("")
    try {
      const weekStart = startOfWeek(date, { weekStartsOn: 1 })
      const weekEnd = endOfWeek(date, { weekStartsOn: 1 })
      const weekDays = eachDayOfInterval({ start: weekStart, end: weekEnd })

      const results = await Promise.all(
        weekDays.map(async (day) => {
          const res = await apiClient.getLessons(day.getFullYear(), day.getMonth() + 1, day.getDate())
          return [day.toISOString().split("T")[0], res.data] as [string, Lesson[]]
        }),
      )
      const weekData = Object.fromEntries(results)
      setWeekLessons(weekData)
      const allLessons = results.flatMap(([, ls]) => ls)
      loadNotesForLessons(allLessons)
    } catch (error) {
      setError("Не удалось загрузить расписание на неделю")
    } finally {
      setIsLoading(false)
    }
  }

  const loadNotesForLessons = async (lessonList: Lesson[]) => {
    if (lessonList.length === 0) return
    try {
      const uuids = lessonList.map((l) => l.uuid)
      const resp = await apiClient.getNotes(uuids)
      const map: Record<string, Note> = {}
      for (const note of resp.data) map[note.lesson_uuid] = note
      setNotes((prev) => ({ ...prev, ...map }))
    } catch { /* игнорируем */ }
  }

  useEffect(() => {
    if (activeTab === "day") loadLessons(selectedDate)
    else loadWeekLessons(selectedDate)
  }, [selectedDate, activeTab])

  const navigateWeek = (direction: "prev" | "next") => setSelectedDate((prev) => (direction === "prev" ? subDays(prev, 7) : addDays(prev, 7)))

  const renderLessonCard = (lesson: Lesson) => {
    const nowSec = Math.floor(Date.now() / 1000)
    const isCurrent = nowSec >= lesson.start && nowSec <= lesson.end
    const hasNote = !!notes[lesson.uuid]
    const isPastOrCurrent = nowSec >= lesson.start
    const displayStatus = (parentMode && isPastOrCurrent) ? 3 : lesson.status

    return (
      <Card
        key={lesson.uuid}
        className={`mb-3 py-0 cursor-pointer hover:shadow-md transition-shadow overflow-hidden ${isCurrent ? 'bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800' : ''}`}
        onClick={() => handleLessonClick(lesson)}
      >
      <CardContent className="p-4">
        <div className="flex items-start justify-between mb-3">
          <div className="flex-1">
            <div className="flex justify-between">
              <div className="flex items-center gap-2 mb-2">
                <Badge className={getLessonTypeColor(lesson.type)}>{getLessonTypeText(lesson.type)}</Badge>
                <Badge variant={getAttendedTypeColor(displayStatus)}>{getAttendedTypeText(displayStatus)}</Badge>
              </div>
              <div className="flex items-center gap-2 mb-2">
                <Badge className="text-white items-center">
                  {formatTime(lesson.start)} - {formatTime(lesson.end)}
                </Badge>
              </div>
            </div>
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-lg mb-1 pr-2">{lesson.title}</h3>
              <Button
                variant="ghost"
                size="icon"
                className={`h-8 w-8 shrink-0 ${hasNote ? 'text-primary' : 'text-muted-foreground'}`}
                onClick={(e) => handleNoteClick(e, lesson)}
              >
                {hasNote ? <StickyNote className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
              </Button>
            </div>
          </div>
        </div>

        <div className="space-y-3">
          <div className="flex items-center gap-2 text-sm text-foreground bg-muted/30 p-2.5 rounded-lg border">
            <MapPin className="h-4 w-4 text-primary shrink-0" />
            <span className="truncate font-medium">
              {lesson.auditorium} {lesson.campus ? `• Корпус ${lesson.campus}` : ''}
            </span>
          </div>

          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <User className="h-4 w-4 shrink-0" />
            <span>{lesson.teacher}</span>
          </div>
          {lesson.groups?.length > 1 && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Users className="h-4 w-4 shrink-0" />
                <span>{lesson.groups.join(", ")}</span>
              </div>
          )}
          {hasNote && (
            <div className="flex items-start gap-2 text-sm text-primary/80 border-t pt-2 mt-2">
              <StickyNote className="h-3.5 w-3.5 mt-0.5 shrink-0" />
              <span className="line-clamp-2">{notes[lesson.uuid].note_text}</span>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
    )
  }

  const renderDayView = () => (
    <div className="space-y-4">
      <Card className="py-0">
        <CardContent className="p-4">
          {(() => {
            const weekStart = startOfWeek(selectedDate, { weekStartsOn: 1 })
            const weekEnd = endOfWeek(selectedDate, { weekStartsOn: 1 })
            const days = Array.from({ length: 7 }, (_, i) => addDays(weekStart, i))
            const weekdayShort = ["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"]

            return (
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <Button variant="ghost" size="icon" onClick={() => navigateWeek("prev")}><ChevronLeft className="h-4 w-4" /></Button>
                  <div className="text-center">
                    <h2 className="text-lg font-semibold select-none cursor-pointer" onClick={handleSecretTap}>
                      {formatDate(weekStart)} - {formatDate(weekEnd)}
                    </h2>
                  </div>
                  <Button variant="ghost" size="icon" onClick={() => navigateWeek("next")}><ChevronRight className="h-4 w-4" /></Button>
                </div>
                <div className="grid grid-cols-7 gap-2">
                  {days.map((day, idx) => {
                    const isActive = isSameDay(day, selectedDate)
                    return (
                      <Button
                        key={idx}
                        variant={isActive ? "default" : "outline"}
                        className="flex flex-col items-center justify-center gap-0"
                        onClick={() => setSelectedDate(day)}
                      >
                        <span className="text-xs font-medium">{weekdayShort[idx]}</span>
                        <span className="text-base leading-none">{format(day, "d", { locale: ru })}</span>
                      </Button>
                    )
                  })}
                </div>
              </div>
            )
          })()}
        </CardContent>
      </Card>

      {isLoading ? (
        <Card><CardContent className="p-8 text-center"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4"></div></CardContent></Card>
      ) : error ? (
        <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>
      ) : lessons.length === 0 ? (
        <Card><CardContent className="p-8 text-center"><Calendar className="h-12 w-12 text-muted-foreground mx-auto mb-4" /><h3 className="text-lg font-semibold mb-2">Занятий нет</h3></CardContent></Card>
      ) : (
        <div>{lessons.map(renderLessonCard)}</div>
      )}
    </div>
  )

  const renderWeekView = () => {
    const weekStart = startOfWeek(selectedDate, { weekStartsOn: 1 })
    const weekEnd = endOfWeek(selectedDate, { weekStartsOn: 1 })
    const weekDays = eachDayOfInterval({ start: weekStart, end: weekEnd })

    return (
      <div className="space-y-4">
        <Card>
          <CardContent className="p-4 py-0">
            <div className="flex items-center justify-between">
              <Button variant="ghost" size="icon" onClick={() => navigateWeek("prev")}><ChevronLeft className="h-4 w-4" /></Button>
              <div className="text-center"><h2 className="text-lg font-semibold select-none cursor-pointer" onClick={handleSecretTap}>{formatDate(weekStart)} - {formatDate(weekEnd)}</h2></div>
              <Button variant="ghost" size="icon" onClick={() => navigateWeek("next")}><ChevronRight className="h-4 w-4" /></Button>
            </div>
          </CardContent>
        </Card>

        {isLoading ? (
          <Card><CardContent className="p-8 text-center"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4"></div></CardContent></Card>
        ) : error ? (
          <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>
        ) : (
          <div className="space-y-4">
            {weekDays.map((day) => {
              const dayKey = day.toISOString().split("T")[0]
              const dayLessons = weekLessons[dayKey] || []
              const isToday = isSameDay(day, new Date())

              return (
                <Card key={dayKey} className={isToday ? "ring-2 ring-primary" : ""}>
                  <CardHeader className="pb-3">
                    <div className="flex items-center justify-between">
                      <div>
                        <CardTitle className="text-base capitalize">{formatWeekday(day)}{isToday && <Badge className="ml-2">Сегодня</Badge>}</CardTitle>
                        <CardDescription className="capitalize">{formatDate(day)}</CardDescription>
                      </div>
                      <Badge variant="outline">{dayLessons.length} занятий</Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="pt-0">
                    {dayLessons.length === 0 ? (
                      <p className="text-sm text-muted-foreground text-center py-4">Занятий нет</p>
                    ) : (
                      <div className="space-y-2">
                        {dayLessons.map((lesson) => {
                          const nowSec = Math.floor(Date.now() / 1000)
                          const isCurrent = isToday && nowSec >= lesson.start && nowSec <= lesson.end
                          const displayStatus = (parentMode && nowSec >= lesson.start) ? 3 : lesson.status

                          return (
                            <div
                              key={lesson.uuid}
                              className={`flex items-center justify-between p-3 rounded-lg border cursor-pointer hover:bg-muted/80 transition-colors ${isCurrent ? 'bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800' : 'bg-muted/50'}`}
                              onClick={() => handleLessonClick(lesson)}
                            >
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2 mb-1">
                                  <Badge className={getLessonTypeColor(lesson.type)}>{lesson.type}</Badge>
                                  <Badge variant={getAttendedTypeColor(displayStatus)} className="h-5 px-1.5 text-[10px]">{getAttendedTypeText(displayStatus)}</Badge>
                                </div>
                                <div className="font-medium text-sm truncate mb-1">{lesson.title}</div>
                                
                                <div className="flex flex-col gap-1.5 mt-2">
                                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                                    <Clock className="h-3 w-3 shrink-0" />
                                    {formatTime(lesson.start)} - {formatTime(lesson.end)}
                                  </div>
                                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                                    <MapPin className="h-3 w-3 shrink-0" />
                                    <span className="truncate">{lesson.auditorium}</span>
                                  </div>
                                </div>
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}
      </div>
    )
  }

  return (
      <div className="min-h-screen bg-background relative">
        <StudentInfoHeader/>
        <main className="container px-4 py-6 pb-20">
          <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
            <TabsList className="grid w-full grid-cols-2 rounded-lg"><TabsTrigger value="day">День</TabsTrigger><TabsTrigger value="week">Неделя</TabsTrigger></TabsList>
            <TabsContent value="day">{renderDayView()}</TabsContent>
            <TabsContent value="week">{renderWeekView()}</TabsContent>
          </Tabs>
        </main>
        <AttendanceModal lesson={selectedLesson} isOpen={isAttendanceModalOpen} onClose={() => setIsAttendanceModalOpen(false)} />
        <NoteModal lesson={noteLesson} isOpen={isNoteModalOpen} existingNote={noteLesson ? (notes[noteLesson.uuid] ?? null) : null} onClose={() => setIsNoteModalOpen(false)} onSaved={handleNoteSaved} />
      </div>
  )
}

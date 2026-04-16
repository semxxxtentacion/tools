"use client"

import { useState } from "react"
import { apiClient, type Student } from "@/lib/api"

export function useStudentConnections() {
  const [connectedStudents, setConnectedStudents] = useState<Student[]>([])
  const [connectedToUser, setConnectedToUser] = useState<Student[]>([])
  const [pendingRequests, setPendingRequests] = useState<Student[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadConnectedStudents = async () => {
    setIsLoading(true)
    setError(null)

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
    setError(null)

    try {
      const response = await apiClient.listConnectedToUser()
      setConnectedToUser(response.data)
    } catch (error) {
      setError("Не удалось загрузить список пользователей")
    } finally {
      setIsLoading(false)
    }
  }

  const loadPendingRequests = async () => {
    try {
      const response = await apiClient.getPendingConnections()
      setPendingRequests(response.data ?? [])
    } catch (error) {
      console.error("Не удалось загрузить запросы на подключение", error)
    }
  }

  const loadAll = async () => {
    await Promise.all([loadConnectedStudents(), loadConnectedToUser(), loadPendingRequests()])
  }

  const findStudent = async (email: string): Promise<Student | null> => {
    try {
      const response = await apiClient.findStudent(email)
      return response.data
    } catch (error) {
      throw new Error("Студент не найден")
    }
  }

  const connectStudent = async (input: string) => {
    try {
      await apiClient.connectStudent(input)
      await loadAll()
    } catch (error) {
      throw new Error("Не удалось добавить студента")
    }
  }

  const acceptRequest = async (fromUserId: string) => {
    try {
      await apiClient.acceptConnection(fromUserId)
      await loadAll()
    } catch (error) {
      throw new Error("Не удалось принять запрос")
    }
  }

  const declineRequest = async (fromUserId: string) => {
    try {
      await apiClient.declineConnection(fromUserId)
      await loadPendingRequests()
    } catch (error) {
      throw new Error("Не удалось отклонить запрос")
    }
  }

  const toggleStudent = async (email: string) => {
    try {
      await apiClient.toggleConnectedStudent(email)
      await loadConnectedStudents()
    } catch (error) {
      throw new Error("Не удалось изменить статус студента")
    }
  }

  const disconnectStudent = async (email: string) => {
    try {
      await apiClient.disconnectStudent(email)
      await loadAll()
    } catch (error) {
      throw new Error("Не удалось отвязать студента")
    }
  }

  const disconnectFromUser = async (email: string) => {
    try {
      await apiClient.disconnectFromUser(email)
      await loadAll()
    } catch (error) {
      throw new Error("Не удалось отвязаться от пользователя")
    }
  }

  const getActiveStudents = () => {
    return connectedStudents.filter((student: Student) => student.enabled)
  }

  const getInactiveStudents = () => {
    return connectedStudents.filter((student: Student) => !student.enabled)
  }

  return {
    connectedStudents,
    connectedToUser,
    pendingRequests,
    isLoading,
    error,
    loadConnectedStudents,
    loadConnectedToUser,
    loadPendingRequests,
    loadAll,
    findStudent,
    connectStudent,
    acceptRequest,
    declineRequest,
    toggleStudent,
    disconnectStudent,
    disconnectFromUser,
    getActiveStudents,
    getInactiveStudents,
  }
}

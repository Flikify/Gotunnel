import axios from 'axios'
import type { ClientConfig, ClientStatus, ClientDetail, ServerStatus } from '../types'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

export const getServerStatus = () => api.get<ServerStatus>('/status')
export const getClients = () => api.get<ClientStatus[]>('/clients')
export const getClient = (id: string) => api.get<ClientDetail>(`/client/${id}`)
export const addClient = (client: ClientConfig) => api.post('/clients', client)
export const updateClient = (id: string, client: ClientConfig) => api.put(`/client/${id}`, client)
export const deleteClient = (id: string) => api.delete(`/client/${id}`)
export const reloadConfig = () => api.post('/config/reload')

export default api
